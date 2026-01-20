package service

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
)

// EmailService is responsible for the business logic around emails.
// It handles template rendering and email formatting, but does NOT
// compute strategy results - those are passed in as domain objects.
type EmailService interface {
	// SendStrategySummaryEmail sends an email to a user with their
	// strategy summary results. The strategyResults are pre-computed
	// domain objects containing what each strategy would buy.
	SendStrategySummaryEmail(
		user *model.UserAccount,
		strategyResults []domain.StrategySummaryResult,
	) error

	// GenerateStrategySummaryEmail generates the email content for a user's
	// strategy results. Returns the subject and HTML body.
	// This is used internally by SendStrategySummaryEmail but can also
	// be called separately for testing/preview purposes.
	GenerateStrategySummaryEmail(
		user *model.UserAccount,
		strategyResults []domain.StrategySummaryResult,
	) (string, string, error)
}

type emailServiceHandler struct {
	EmailRepository repository.EmailRepository
}

// Typed view-models for template rendering.
// Use these fields instead of domain to keep the template clean.
type strategySummaryEmailData struct {
	UserName    string
	RunDate     string
	TradingDate string
	Strategies  []strategySummaryEmailStrategy
}

type strategySummaryEmailStrategy struct {
	StrategyName string
	StrategyURL  string
	Error        string
	Assets       []strategySummaryEmailAsset
}

type strategySummaryEmailAsset struct {
	Symbol      string
	Weight      float64
	FactorScore float64
	Price       float64
}

func NewEmailService(
	emailRepository repository.EmailRepository,
) EmailService {
	return &emailServiceHandler{
		EmailRepository: emailRepository,
	}
}

func (h *emailServiceHandler) SendStrategySummaryEmail(
	user *model.UserAccount,
	strategyResults []domain.StrategySummaryResult,
) error {
	if user.Email == nil || *user.Email == "" {
		return fmt.Errorf("user has no email address")
	}

	subject, htmlBody, err := h.GenerateStrategySummaryEmail(user, strategyResults)
	if err != nil {
		return fmt.Errorf("failed to generate email content: %w", err)
	}

	err = h.EmailRepository.SendEmail(*user.Email, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (h *emailServiceHandler) GenerateStrategySummaryEmail(
	user *model.UserAccount,
	strategyResults []domain.StrategySummaryResult,
) (string, string, error) {
	if len(strategyResults) == 0 {
		return "", "", fmt.Errorf("no strategy results provided")
	}

	// Trading date is the date the strategy computations are based on.
	tradingDate := strategyResults[0].Date
	// Run date is when we send the email (cron run date).
	runDate := time.Now().UTC()

	// Convert domain objects to template data format
	templateData := h.convertToTemplateData(user, strategyResults, runDate, tradingDate)

	// Render the template
	htmlBody, err := h.renderTemplate("strategy_summary", templateData)
	if err != nil {
		return "", "", fmt.Errorf("failed to render template: %w", err)
	}

	// Generate subject line
	subject := fmt.Sprintf("Your Strategy Updates for %s", runDate.Format("January 2, 2006"))

	return subject, htmlBody, nil
}

// convertToTemplateData converts domain objects to the format expected by the template
func (h *emailServiceHandler) convertToTemplateData(
	user *model.UserAccount,
	strategyResults []domain.StrategySummaryResult,
	runDate time.Time,
	tradingDate time.Time,
) strategySummaryEmailData {
	userName := "there"
	if user.FirstName != nil {
		userName = *user.FirstName
	}

	strategies := []strategySummaryEmailStrategy{}
	for _, result := range strategyResults {
		strategyURL := fmt.Sprintf("https://factor.trade/backtest?id=%s", result.StrategyID.String())

		if result.Error != nil {
			strategies = append(strategies, strategySummaryEmailStrategy{
				StrategyName: result.StrategyName,
				StrategyURL:  strategyURL,
				Error:        result.Error.Error(),
				Assets:       []strategySummaryEmailAsset{},
			})
			continue
		}

		assets := []strategySummaryEmailAsset{}
		for _, asset := range result.Assets {
			assets = append(assets, strategySummaryEmailAsset{
				Symbol:      asset.Symbol,
				Weight:      asset.Weight * 100, // Convert to percentage
				FactorScore: asset.FactorScore,
				Price:       asset.LastPrice.InexactFloat64(),
			})
		}

		// Sort descending by weight so the table shows highest allocation first.
		sort.Slice(assets, func(i, j int) bool {
			return assets[i].Weight > assets[j].Weight
		})

		strategies = append(strategies, strategySummaryEmailStrategy{
			StrategyName: result.StrategyName,
			StrategyURL:  strategyURL,
			Assets:       assets,
		})
	}

	return strategySummaryEmailData{
		UserName:    userName,
		RunDate:     runDate.Format("January 2, 2006"),
		TradingDate: tradingDate.Format("January 2, 2006"),
		Strategies:  strategies,
	}
}

// findTemplatePath tries multiple possible locations for template files
func findTemplatePath(templateName string) (string, error) {
	wd, _ := os.Getwd()

	possiblePaths := []string{
		filepath.Join("templates", templateName+".html"),                // From project root
		filepath.Join("..", "templates", templateName+".html"),          // From internal/service/
		filepath.Join("../..", "templates", templateName+".html"),       // From deeper nested dirs
		filepath.Join(wd, "templates", templateName+".html"),            // Absolute from current dir
		filepath.Join("/go/src/app", "templates", templateName+".html"), // Lambda path
	}

	for _, templatePath := range possiblePaths {
		if _, err := os.Stat(templatePath); err == nil {
			return templatePath, nil
		}
	}

	return "", fmt.Errorf("template %s not found in any of these locations: %v", templateName, possiblePaths)
}

// renderTemplate loads and renders a template with the given data
func (h *emailServiceHandler) renderTemplate(templateName string, templateData interface{}) (string, error) {
	// Find template file
	templatePath, err := findTemplatePath(templateName)
	if err != nil {
		return "", err
	}

	// Read template content
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	// Parse the template
	tmpl, err := template.New(templateName).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Execute template with data
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}
