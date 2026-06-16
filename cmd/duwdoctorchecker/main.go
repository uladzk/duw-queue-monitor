package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/uladzk/duw-queue-monitor/internal/duwdoctor"
	"github.com/uladzk/duw-queue-monitor/internal/logger"
	"github.com/uladzk/duw-queue-monitor/internal/notifications"
	"github.com/uladzk/duw-queue-monitor/internal/queuemonitor"

	"github.com/caarlos0/env/v11"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config is the duw-doctor checker configuration (env-driven, like the rest of the project).
type Config struct {
	DuwApiUrl               string `env:"DUW_API_URL" envDefault:"https://rezerwacje.duw.pl/status_kolejek/query.php?status="`
	MonitorQueueID          int    `env:"MONITOR_QUEUE_ID" envDefault:"24"`
	MonitorQueueCity        string `env:"MONITOR_QUEUE_CITY" envDefault:"Wrocław"`
	TelegramChannel         string `env:"TELEGRAM_CHANNEL" envDefault:"duw_queue_updates"`
	WorkingHourStartUTC     int    `env:"WORKING_HOUR_START_UTC" envDefault:"5"`
	WorkingHourEndUTC       int    `env:"WORKING_HOUR_END_UTC" envDefault:"17"`
	CheckerPadMinutes       int    `env:"CHECKER_PAD_MINUTES" envDefault:"30"`
	DebounceK               int    `env:"DEBOUNCE_K" envDefault:"3"`
	CooldownMinutes         int    `env:"COOLDOWN_MINUTES" envDefault:"60"`
	DuplicateFloodThreshold int    `env:"DUPLICATE_FLOOD_THRESHOLD" envDefault:"5"`
	DuplicateWindowMinutes  int    `env:"DUPLICATE_WINDOW_MINUTES" envDefault:"10"`
	LogSinceHours           int    `env:"LOG_SINCE_HOURS" envDefault:"3"`
	StateConfigMap          string `env:"STATE_CONFIGMAP" envDefault:"duw-doctor-state"`
	PodNamespace            string `env:"POD_NAMESPACE" envDefault:"default"`
	MonitorDeployment       string `env:"MONITOR_DEPLOYMENT" envDefault:"queue-monitor"`
	MonitorPodSelector      string `env:"MONITOR_POD_SELECTOR" envDefault:"app=queue-monitor"`
	HTTPTimeoutSeconds      int    `env:"CHECKER_HTTP_TIMEOUT_SECONDS" envDefault:"10"`
	AlertChatID             string `env:"NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID,required"`
	DryRun                  bool   `env:"DRY_RUN" envDefault:"false"`
}

const stateKey = "state.json"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var logCfg logger.Config
	if err := env.Parse(&logCfg); err != nil {
		return fmt.Errorf("parse logger config: %w", err)
	}
	log := logger.NewLogger(&logCfg)

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return fmt.Errorf("parse checker config: %w", err)
	}
	var tgCfg notifications.TelegramConfig
	if err := env.Parse(&tgCfg); err != nil {
		return fmt.Errorf("parse telegram config: %w", err)
	}

	now := time.Now().UTC()
	httpClient := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second}

	// 1. DUW API (source of truth) — reuse StatusCollector (empty UA + retry + negative clamp).
	qmCfg := &queuemonitor.QueueMonitorConfig{
		StatusMonitoredQueueId:    cfg.MonitorQueueID,
		StatusMonitoredQueueCity:  cfg.MonitorQueueCity,
		StatusApiUrl:              cfg.DuwApiUrl,
		StatusCheckTimeoutMs:      4000,
		StatusCheckMaxAttempts:    3,
		StatusCheckAttemptDelayMs: 500,
	}
	queue, duwErr := queuemonitor.NewStatusCollector(qmCfg, httpClient, log).GetQueueStatus(ctx)
	schemaValid := duwErr == nil
	var expected duwdoctor.State
	if schemaValid {
		expected = duwdoctor.ExpectedFromQueue(*queue)
	} else {
		log.Warn("DUW status fetch failed (treated as contract drift / unreachable)", "error", duwErr)
	}

	// 2. Channel (observed end-user truth).
	posts := fetchChannel(ctx, httpClient, cfg.TelegramChannel, log)
	observed, _, observedOK := duwdoctor.ObservedState(posts)
	dupWindow := time.Duration(cfg.DuplicateWindowMinutes) * time.Minute
	run := duwdoctor.MaxIdenticalRun(posts, now, dupWindow)

	// 3. Cluster client (state ConfigMap + snapshot).
	clientset, err := buildClientset()
	if err != nil {
		return fmt.Errorf("build k8s client: %w", err)
	}
	cmAPI := clientset.CoreV1().ConfigMaps(cfg.PodNamespace)
	cm, err := cmAPI.Get(ctx, cfg.StateConfigMap, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get state configmap %q: %w", cfg.StateConfigMap, err)
	}
	prev, err := duwdoctor.ParseState(cm.Data[stateKey])
	if err != nil {
		log.Warn("could not parse prior state, starting from zero", "error", err)
		prev = duwdoctor.DoctorState{}
	}

	zone := duwdoctor.ZoneAt(now, cfg.WorkingHourStartUTC, cfg.WorkingHourEndUTC, cfg.CheckerPadMinutes)
	result := duwdoctor.Evaluate(duwdoctor.CheckInput{
		Now:             now,
		Zone:            zone,
		SchemaValid:     schemaValid,
		Expected:        expected,
		Observed:        observed,
		ObservedOK:      observedOK,
		MaxIdenticalRun: run,
		Prev:            prev,
		DebounceK:       cfg.DebounceK,
		Cooldown:        time.Duration(cfg.CooldownMinutes) * time.Minute,
		FloodThreshold:  cfg.DuplicateFloodThreshold,
	})

	log.Info("check evaluated",
		"zone", zone, "schemaValid", schemaValid, "expected", expected,
		"observed", observed, "observedOK", observedOK, "maxIdenticalRun", run,
		"escalate", result.Escalate, "reason", result.Reason)

	// 4. Persist next state.
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	if raw, mErr := result.Next.Marshal(); mErr == nil {
		cm.Data[stateKey] = raw
		if _, uErr := cmAPI.Update(ctx, cm, metav1.UpdateOptions{}); uErr != nil {
			log.Error("failed to persist state", uErr)
		}
	}

	// 5. Escalate.
	if !result.Escalate {
		return nil
	}
	snapshot := captureSnapshot(ctx, clientset, &cfg, log)
	if cfg.DryRun {
		log.Info("DRY_RUN: would escalate — alert NOT sent", "reason", result.Reason)
		fmt.Printf("=== DUW-DOCTOR SNAPSHOT (DRY_RUN) ===\n%s\n", snapshot)
		return nil
	}

	text := alertMessage(result.Reason, expected, observed, observedOK, posts, now)
	notifier := notifications.NewTelegramNotifier(&tgCfg, log, httpClient)
	if err := notifier.SendMessage(ctx, cfg.AlertChatID, text); err != nil {
		log.Error("failed to send telegram alert", err)
	}
	fmt.Printf("=== DUW-DOCTOR SNAPSHOT ===\n%s\n", snapshot)
	return nil
}

// fetchChannel scrapes t.me/s with a browser UA and a small retry; returns nil posts on failure.
func fetchChannel(ctx context.Context, client *http.Client, channel string, log *logger.Logger) []duwdoctor.Post {
	url := "https://t.me/s/" + channel
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; duw-doctor-checker)")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		body, rErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rErr != nil || resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d: %v", resp.StatusCode, rErr)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		posts, pErr := duwdoctor.ParseChannel(string(body))
		if pErr != nil {
			lastErr = pErr
			continue
		}
		return posts
	}
	log.Warn("channel fetch failed (observed state unavailable)", "error", lastErr)
	return nil
}

func buildClientset() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// captureSnapshot collects read-only cluster context for the alert/issue. Best-effort:
// any failure is logged and skipped, never blocking the alert.
func captureSnapshot(ctx context.Context, cs *kubernetes.Clientset, cfg *Config, log *logger.Logger) string {
	var b strings.Builder
	ns := cfg.PodNamespace

	pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: cfg.MonitorPodSelector})
	if err != nil {
		fmt.Fprintf(&b, "pods: error: %v\n", err)
	} else {
		for _, p := range pods.Items {
			restarts := int32(0)
			for _, st := range p.Status.ContainerStatuses {
				restarts += st.RestartCount
			}
			fmt.Fprintf(&b, "pod %s phase=%s restarts=%d\n", p.Name, p.Status.Phase, restarts)
			logs := podLogs(ctx, cs, ns, p.Name, int64(cfg.LogSinceHours*3600))
			fmt.Fprintf(&b, "--- logs (since %dh) ---\n%s\n", cfg.LogSinceHours, logs)
		}
	}

	if dep, err := cs.AppsV1().Deployments(ns).Get(ctx, cfg.MonitorDeployment, metav1.GetOptions{}); err != nil {
		fmt.Fprintf(&b, "deployment: error: %v\n", err)
	} else {
		s := dep.Status
		fmt.Fprintf(&b, "deployment %s replicas=%d ready=%d available=%d updated=%d\n",
			dep.Name, s.Replicas, s.ReadyReplicas, s.AvailableReplicas, s.UpdatedReplicas)
	}

	if evs, err := cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Fprintf(&b, "events: error: %v\n", err)
	} else {
		fmt.Fprintf(&b, "--- recent events ---\n")
		items := evs.Items
		start := 0
		if len(items) > 15 {
			start = len(items) - 15
		}
		for _, e := range items[start:] {
			fmt.Fprintf(&b, "%s %s/%s: %s\n", e.Type, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Message)
		}
	}
	return b.String()
}

func podLogs(ctx context.Context, cs *kubernetes.Clientset, ns, name string, sinceSeconds int64) string {
	req := cs.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{SinceSeconds: &sinceSeconds})
	rc, err := req.Stream(ctx)
	if err != nil {
		return fmt.Sprintf("logs error: %v", err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Sprintf("logs read error: %v", err)
	}
	return string(body)
}

// alertMessage builds the concise HTML Telegram alert (dynamic values escaped).
func alertMessage(reason string, expected, observed duwdoctor.State, observedOK bool, posts []duwdoctor.Post, now time.Time) string {
	obs := "none"
	if observedOK {
		obs = string(observed)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<b>🩺 duw-doctor: %s</b>\n", html.EscapeString(reason))
	fmt.Fprintf(&b, "expected=<b>%s</b> observed=<b>%s</b>\n", html.EscapeString(string(expected)), html.EscapeString(obs))
	fmt.Fprintf(&b, "at %s UTC\n\n", now.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "last channel posts:\n")
	start := 0
	if len(posts) > 3 {
		start = len(posts) - 3
	}
	for _, p := range posts[start:] {
		fmt.Fprintf(&b, "• %s %s\n", p.At.Format("15:04"), html.EscapeString(p.Text))
	}
	return b.String()
}
