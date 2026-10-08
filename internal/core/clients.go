package core

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"karina/internal/config"
	"karina/internal/domain"
	"karina/internal/transcripts"
)

// Claude Code usage seen through the lens of who the work was for: clients,
// time worked, reports and budgets. Everything here reads the local
// transcripts only; nothing goes over the network.

const (
	minIdleGapMinutes = 1
	maxIdleGapMinutes = 60
	// budgetWarnPercent is the first budget alert; the second fires at 100.
	budgetWarnPercent = 80
)

// ClaudeCodeUsage reads local Claude Code transcripts
// (~/.claude/projects/**/*.jsonl) and reports real token usage per project
// and per day for the given span. Unlike every provider adapter, this
// reads no network at all — it is the one number Karina can show with zero
// ambiguity, straight from the same files Claude Code itself writes.
func (s *Service) ClaudeCodeUsage(span domain.HistorySpan) (transcripts.Summary, error) {
	return s.usageFor(span.Start(time.Now()), time.Time{})
}

// usageFor scans a time range and labels the projects with their clients.
func (s *Service) usageFor(since, until time.Time) (transcripts.Summary, error) {
	root, err := s.transcriptsRoot()
	if err != nil {
		return transcripts.Summary{}, err
	}
	s.mu.Lock()
	gap := time.Duration(s.cfg.IdleGap()) * time.Minute
	byPath := make(map[string]string, len(s.cfg.ProjectClients))
	for path, client := range s.cfg.ProjectClients {
		byPath[path] = client
	}
	byFolder := make(map[string]string, len(s.cfg.FolderClients))
	for folder, client := range s.cfg.FolderClients {
		byFolder[folder] = client
	}
	s.mu.Unlock()

	summary, err := transcripts.ScanRange(root, transcripts.Range{Since: since, Until: until, IdleGap: gap})
	if err != nil {
		return transcripts.Summary{}, err
	}
	summary.AssignClients(byPath, byFolder)
	return summary, nil
}

func (s *Service) transcriptsRoot() (string, error) {
	if s.transcriptsDir != "" {
		return s.transcriptsDir, nil
	}
	return transcripts.DefaultRoot()
}

// SetIdleGapMinutes sets the pause that ends a stretch of work when
// measuring active time.
func (s *Service) SetIdleGapMinutes(minutes int) error {
	if minutes < minIdleGapMinutes {
		minutes = minIdleGapMinutes
	}
	if minutes > maxIdleGapMinutes {
		minutes = maxIdleGapMinutes
	}
	s.mu.Lock()
	s.cfg.IdleGapMinutes = minutes
	s.mu.Unlock()
	return config.Save(s.cfgPath, s.cfg)
}

// SetSubscriptionInterval sets how often the Claude subscription usage is
// read. It is clamped to a safe range: the endpoint rate-limits bursts.
func (s *Service) SetSubscriptionInterval(seconds int) error {
	if seconds < config.MinSubscriptionSeconds {
		seconds = config.MinSubscriptionSeconds
	}
	if seconds > maxIntervalSeconds {
		seconds = maxIntervalSeconds
	}
	s.mu.Lock()
	s.cfg.SubscriptionIntervalSeconds = seconds
	s.mu.Unlock()
	return config.Save(s.cfgPath, s.cfg)
}

// SetReportBusinessName sets the name printed at the top of client reports.
func (s *Service) SetReportBusinessName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) > 80 {
		return fmt.Errorf("el nombre es demasiado largo (máx. 80)")
	}
	s.mu.Lock()
	s.cfg.ReportBusinessName = name
	s.mu.Unlock()
	return config.Save(s.cfgPath, s.cfg)
}

// reportRange turns a report period into a time range and a Spanish label.
// Besides the usual spans it knows calendar months, which is how client
// work is normally reported.
func reportRange(period string, now time.Time) (since, until time.Time, label string, err error) {
	y, m, _ := now.Date()
	monthStart := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	switch period {
	case "this_month":
		return monthStart, time.Time{}, monthLabel(monthStart) + " (hasta hoy)", nil
	case "last_month":
		prev := monthStart.AddDate(0, -1, 0)
		return prev, monthStart, monthLabel(prev), nil
	case string(domain.SpanToday):
		return domain.SpanToday.Start(now), time.Time{}, "Hoy, " + now.Format("02/01/2006"), nil
	case string(domain.Span7d):
		return domain.Span7d.Start(now), time.Time{}, "Últimos 7 días", nil
	case string(domain.Span30d):
		return domain.Span30d.Start(now), time.Time{}, "Últimos 30 días", nil
	}
	return time.Time{}, time.Time{}, "", fmt.Errorf("periodo desconocido: %q", period)
}

var monthNames = [...]string{
	"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
}

func monthLabel(t time.Time) string {
	return fmt.Sprintf("%s de %d", monthNames[t.Month()-1], t.Year())
}

// ClientReport is the data of a client report for one period.
type ClientReport struct {
	Period       string              `json:"period"`
	PeriodLabel  string              `json:"period_label"`
	BusinessName string              `json:"business_name"`
	GeneratedAt  string              `json:"generated_at"`
	Summary      transcripts.Summary `json:"summary"`
}

// ClientReport gathers the Claude Code usage of a period grouped by client:
// time worked, tokens and estimated cost. The UI prints it to PDF.
func (s *Service) ClientReport(period string) (ClientReport, error) {
	now := time.Now()
	since, until, label, err := reportRange(period, now)
	if err != nil {
		return ClientReport{}, err
	}
	summary, err := s.usageFor(since, until)
	if err != nil {
		return ClientReport{}, err
	}
	s.mu.Lock()
	business := s.cfg.ReportBusinessName
	s.mu.Unlock()
	return ClientReport{
		Period:       period,
		PeriodLabel:  label,
		BusinessName: business,
		GeneratedAt:  now.Format(time.RFC3339),
		Summary:      summary,
	}, nil
}

// ExportClientReportCSV renders the Claude Code usage of a period grouped by
// client and project: hours worked, tokens and estimated cost at API list
// prices. It is the report a freelancer or agency hands to (or keeps per)
// client.
func (s *Service) ExportClientReportCSV(period string) ([]byte, error) {
	report, err := s.ClientReport(period)
	if err != nil {
		return nil, err
	}
	summary := report.Summary
	if !summary.Available || len(summary.Projects) == 0 {
		return nil, fmt.Errorf("no hay actividad de Claude Code en este periodo")
	}

	projects := append([]transcripts.ProjectUsage(nil), summary.Projects...)
	sort.SliceStable(projects, func(i, j int) bool {
		// Named clients first (alphabetical), unassigned projects last.
		ci, cj := projects[i].Client, projects[j].Client
		if ci != cj {
			if ci == "" || cj == "" {
				return cj == ""
			}
			return ci < cj
		}
		return projects[i].CostUSD > projects[j].CostUSD
	})

	var buf bytes.Buffer
	// BOM so Excel opens the accents correctly.
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"cliente", "proyecto", "ruta", "sesiones", "horas_activas",
		"tokens_entrada", "tokens_salida", "tokens_cache_escritura", "tokens_cache_lectura",
		"tokens_total", "costo_estimado_usd",
	})
	row := func(client, label, path string, sessions int, seconds int64, tk transcripts.Tokens, cost float64) {
		_ = w.Write([]string{
			client, label, path, strconv.Itoa(sessions),
			strconv.FormatFloat(float64(seconds)/3600, 'f', 2, 64),
			strconv.FormatInt(tk.Input, 10),
			strconv.FormatInt(tk.Output, 10),
			strconv.FormatInt(tk.CacheCreation, 10),
			strconv.FormatInt(tk.CacheRead, 10),
			strconv.FormatInt(tk.Total(), 10),
			strconv.FormatFloat(cost, 'f', 2, 64),
		})
	}
	clientLabel := func(name string) string {
		if name == "" {
			return "Sin cliente"
		}
		return name
	}
	for _, p := range projects {
		row(clientLabel(p.Client), p.Label, p.Path, p.Sessions, p.ActiveSeconds, p.Tokens, p.CostUSD)
	}
	for _, c := range summary.Clients {
		row(clientLabel(c.Name), "TOTAL CLIENTE", "", c.Sessions, c.ActiveSeconds, c.Tokens, c.CostUSD)
	}
	row("TOTAL", "", "", 0, summary.ActiveSeconds, summary.Total, summary.CostUSD)
	_ = w.Write([]string{
		"nota",
		fmt.Sprintf("Periodo: %s. Costo estimado a precios de lista de la API de Anthropic al %s; no es una factura. "+
			"Horas activas: tiempo entre respuestas de Claude Code con pausas de hasta %d min.",
			report.PeriodLabel, summary.PricesAsOf, summary.IdleGapMinutes),
	})
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("generar csv: %w", err)
	}
	return buf.Bytes(), nil
}

// ClientBudget is a client's monthly budget against what it has consumed so
// far this calendar month.
type ClientBudget struct {
	Name      string  `json:"name"`
	BudgetUSD float64 `json:"budget_usd"`
	SpentUSD  float64 `json:"spent_usd"`
	Percent   float64 `json:"percent"`
}

// SetClientBudget sets a client's monthly budget in USD (0 removes it).
func (s *Service) SetClientBudget(client string, usd float64) error {
	client = strings.TrimSpace(client)
	if client == "" {
		return fmt.Errorf("indica el cliente")
	}
	if usd < 0 || usd > 10_000_000 || usd != usd {
		return fmt.Errorf("presupuesto inválido")
	}
	s.mu.Lock()
	if usd == 0 {
		delete(s.cfg.ClientBudgets, client)
	} else {
		if s.cfg.ClientBudgets == nil {
			s.cfg.ClientBudgets = map[string]float64{}
		}
		s.cfg.ClientBudgets[client] = usd
	}
	s.mu.Unlock()
	return config.Save(s.cfgPath, s.cfg)
}

// ClientBudgets returns every budget with the client's estimated spend of
// the current calendar month, the most consumed first.
func (s *Service) ClientBudgets() ([]ClientBudget, error) {
	s.mu.Lock()
	budgets := make(map[string]float64, len(s.cfg.ClientBudgets))
	for name, usd := range s.cfg.ClientBudgets {
		budgets[name] = usd
	}
	s.mu.Unlock()
	out := []ClientBudget{}
	if len(budgets) == 0 {
		return out, nil
	}

	since, until, _, _ := reportRange("this_month", time.Now())
	summary, err := s.usageFor(since, until)
	if err != nil {
		return nil, err
	}
	spent := map[string]float64{}
	for _, c := range summary.Clients {
		spent[c.Name] = c.CostUSD
	}
	for name, usd := range budgets {
		b := ClientBudget{Name: name, BudgetUSD: usd, SpentUSD: spent[name]}
		if usd > 0 {
			b.Percent = b.SpentUSD / usd * 100
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Percent != out[j].Percent {
			return out[i].Percent > out[j].Percent
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// checkBudgets alerts once when a client reaches 80% of its monthly budget
// and once more at 100%. What was already announced is kept in the config,
// so restarting Karina does not repeat the alert.
func (s *Service) checkBudgets() {
	s.mu.Lock()
	enabled := s.cfg.AlertsEnabled && len(s.cfg.ClientBudgets) > 0
	s.mu.Unlock()
	if !enabled {
		return
	}
	budgets, err := s.ClientBudgets()
	if err != nil {
		s.logger.Debug("budget check failed", "error", err.Error())
		return
	}
	month := time.Now().Format("2006-01")
	changed := false
	for _, b := range budgets {
		level := 0
		switch {
		case b.Percent >= 100:
			level = 100
		case b.Percent >= budgetWarnPercent:
			level = budgetWarnPercent
		}
		if level == 0 {
			continue
		}
		key := month + "|" + b.Name
		s.mu.Lock()
		already := s.cfg.BudgetAlerts[key]
		if level > already {
			if s.cfg.BudgetAlerts == nil {
				s.cfg.BudgetAlerts = map[string]int{}
			}
			// Drop the marks of previous months while we are here.
			for k := range s.cfg.BudgetAlerts {
				if !strings.HasPrefix(k, month+"|") {
					delete(s.cfg.BudgetAlerts, k)
				}
			}
			s.cfg.BudgetAlerts[key] = level
			changed = true
		}
		s.mu.Unlock()
		if level <= already {
			continue
		}

		title := "Karina · Presupuesto de " + b.Name
		msg := fmt.Sprintf("%.0f%% del presupuesto del mes: $%.2f de $%.2f", b.Percent, b.SpentUSD, b.BudgetUSD)
		if level >= 100 {
			msg = fmt.Sprintf("Presupuesto del mes superado: $%.2f de $%.2f", b.SpentUSD, b.BudgetUSD)
		}
		if err := s.notify(title, msg); err != nil {
			s.logger.Debug("os notification failed", "error", err.Error())
		}
		state := domain.ProviderState{Provider: "claude_code", DisplayName: "Claude Code"}
		s.fireWebhook(title, msg, state, "Presupuesto "+b.Name, b.Percent, time.Time{})
		s.logger.Info("budget alert fired", "client", b.Name, "percent", b.Percent)
		s.emit(Event{Kind: EventThreshold, Message: fmt.Sprintf("%s: %s", b.Name, msg)})
	}
	if changed {
		if err := config.Save(s.cfgPath, s.cfg); err != nil {
			s.logger.Debug("could not save budget alerts", "error", err.Error())
		}
	}
}
