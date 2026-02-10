package statsreporting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UladzK/duw-queue-monitor/internal/dailystats"
)

type StatsReader interface {
	GetByDateRange(ctx context.Context, queueID int, startDate, endDate time.Time) ([]dailystats.QueueDailyStat, error)
}

type MessageSender interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

const (
	msgDailyReport  = "📊 Kolejka <b>%s</b> — podsumowanie dnia:\nObsłużonych klientów: <b>%d</b>\nWydanych biletów: <b>%d</b>"
	msgDailyNoData  = "📊 Kolejka <b>%s</b> — podsumowanie dnia:\nBrak danych za ten dzień."
	msgWeeklyHeader = "📊 Kolejka <b>%s</b> — podsumowanie tygodnia:"
	msgWeeklyDay    = "• %s %s — obsłużono <b>%d</b>, wydano <b>%d</b> biletów"
	msgWeeklyTotal  = "Razem obsłużono: <b>%d</b> klientów\nRazem wydano: <b>%d</b> biletów"
	msgWeeklyNoData = "📊 Kolejka <b>%s</b> — podsumowanie tygodnia:\nBrak danych za ten tydzień."
	msgMonthlyReport = "📊 Kolejka <b>%s</b> — podsumowanie miesiąca:\nŁączna liczba obsłużonych klientów: <b>%d</b>\nŁączna liczba wydanych biletów: <b>%d</b>"
	msgMonthlyNoData = "📊 Kolejka <b>%s</b> — podsumowanie miesiąca:\nBrak danych za ten miesiąc."
)

var polishWeekdays = map[time.Weekday]string{
	time.Monday:    "Pon",
	time.Tuesday:   "Wt",
	time.Wednesday: "Śr",
	time.Thursday:  "Czw",
	time.Friday:    "Pt",
	time.Saturday:  "Sob",
	time.Sunday:    "Ndz",
}

func buildDailyMsg(queueName string, stats []dailystats.QueueDailyStat) string {
	if len(stats) == 0 {
		return fmt.Sprintf(msgDailyNoData, queueName)
	}
	return fmt.Sprintf(msgDailyReport, queueName, stats[0].TicketsServed, stats[0].RegisteredTickets)
}

func buildWeeklyMsg(queueName string, stats []dailystats.QueueDailyStat) string {
	if len(stats) == 0 {
		return fmt.Sprintf(msgWeeklyNoData, queueName)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, msgWeeklyHeader, queueName)

	var totalServed, totalRegistered int32
	for _, s := range stats {
		dayAbbr := polishWeekdays[s.Date.Weekday()]
		dateFmt := s.Date.Format("02.01")
		sb.WriteString("\n")
		fmt.Fprintf(&sb, msgWeeklyDay, dayAbbr, dateFmt, s.TicketsServed, s.RegisteredTickets)
		totalServed += s.TicketsServed
		totalRegistered += s.RegisteredTickets
	}

	sb.WriteString("\n")
	fmt.Fprintf(&sb, msgWeeklyTotal, totalServed, totalRegistered)
	return sb.String()
}

func buildMonthlyMsg(queueName string, stats []dailystats.QueueDailyStat) string {
	if len(stats) == 0 {
		return fmt.Sprintf(msgMonthlyNoData, queueName)
	}

	var totalServed, totalRegistered int32
	for _, s := range stats {
		totalServed += s.TicketsServed
		totalRegistered += s.RegisteredTickets
	}
	return fmt.Sprintf(msgMonthlyReport, queueName, totalServed, totalRegistered)
}

