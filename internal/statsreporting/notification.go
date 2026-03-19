package statsreporting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/uladzk/duw-queue-monitor/internal/dailystats"
)

type StatsReader interface {
	GetByDateRange(ctx context.Context, queueID int, startDate, endDate time.Time) ([]dailystats.QueueDailyStat, error)
}

type MessageSender interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

const (
	msgDailyReport   = "📊 Kolejka <b>%s</b> — podsumowanie dnia:\nWydanych biletów: <b>%d</b>\nPobranych biletów: <b>%d</b>"
	msgDailyNoData   = "📊 Kolejka <b>%s</b> — podsumowanie dnia:\nBrak danych za ten dzień."
	msgWeeklyHeader  = "📊 Kolejka <b>%s</b> — podsumowanie tygodnia:"
	msgWeeklyDay     = "• %s %s — wydano <b>%d</b>, pobrano <b>%d</b> biletów"
	msgWeeklyTotal   = "Razem wydano: <b>%d</b> biletów\nRazem pobrano: <b>%d</b> biletów"
	msgWeeklyNoData  = "📊 Kolejka <b>%s</b> — podsumowanie tygodnia:\nBrak danych za ten tydzień."
	msgMonthlyReport = "📊 Kolejka <b>%s</b> — podsumowanie miesiąca:\nŁączna liczba wydanych biletów: <b>%d</b>\nŁączna liczba pobranych biletów: <b>%d</b>"
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
	return fmt.Sprintf(msgDailyReport, queueName, stats[0].TotalTicketsAvailable, stats[0].TakenTickets)
}

func buildWeeklyMsg(queueName string, stats []dailystats.QueueDailyStat) string {
	if len(stats) == 0 {
		return fmt.Sprintf(msgWeeklyNoData, queueName)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, msgWeeklyHeader, queueName)

	var totalAvailable, totalTaken int32
	for _, s := range stats {
		dayAbbr := polishWeekdays[s.Date.Weekday()]
		dateFmt := s.Date.Format("02.01")
		sb.WriteString("\n")
		fmt.Fprintf(&sb, msgWeeklyDay, dayAbbr, dateFmt, s.TotalTicketsAvailable, s.TakenTickets)
		totalAvailable += s.TotalTicketsAvailable
		totalTaken += s.TakenTickets
	}

	sb.WriteString("\n")
	fmt.Fprintf(&sb, msgWeeklyTotal, totalAvailable, totalTaken)
	return sb.String()
}

func buildMonthlyMsg(queueName string, stats []dailystats.QueueDailyStat) string {
	if len(stats) == 0 {
		return fmt.Sprintf(msgMonthlyNoData, queueName)
	}

	var totalAvailable, totalTaken int32
	for _, s := range stats {
		totalAvailable += s.TotalTicketsAvailable
		totalTaken += s.TakenTickets
	}
	return fmt.Sprintf(msgMonthlyReport, queueName, totalAvailable, totalTaken)
}

