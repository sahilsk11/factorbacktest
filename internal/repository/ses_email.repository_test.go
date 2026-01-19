package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func initializeEmailHandler() (EmailRepository, error) {
	secretsFile := "../../secrets-dev.json"
	f, err := os.ReadFile(secretsFile)
	if err != nil {
		return nil, fmt.Errorf("could not open secrets-dev.json: %w", err)
	}

	type secrets struct {
		SES struct {
			Region    string `json:"region"`
			FromEmail string `json:"fromEmail"`
		} `json:"ses"`
	}

	s := secrets{}
	err = json.Unmarshal(f, &s)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %w", err)
	}

	if s.SES.Region == "" {
		return nil, fmt.Errorf("SES region not found in secrets")
	}
	if s.SES.FromEmail == "" {
		return nil, fmt.Errorf("SES fromEmail not found in secrets")
	}

	repo, err := NewEmailRepository(s.SES.Region, s.SES.FromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to create email repository: %w", err)
	}

	return repo, nil
}

func Test_emailRepositoryHandler_SendEmail(t *testing.T) {
	// Skip by default - set to false to run the test
	if true {
		t.Skip("Skipping email test - set condition to false to run")
	}

	handler, err := initializeEmailHandler()
	require.NoError(t, err)

	testEmail := "sahilkapur.a@gmail.com"
	subject := "Test Email from Factor Backtest"
	body := `
		<html>
			<body>
				<h1>Test Email</h1>
				<p>This is a test email from the Factor Backtest application.</p>
				<p>If you're receiving this, the SES email repository is working correctly!</p>
				<p>Time: <strong>Test sent successfully</strong></p>
			</body>
		</html>
	`

	t.Logf("Attempting to send email to %s", testEmail)
	err = handler.SendEmail(testEmail, subject, body)

	if err != nil {
		t.Logf("ERROR: Failed to send email: %v", err)
		t.Logf("")
		t.Logf("Common issues:")
		t.Logf("1. SES Sandbox Mode: If your SES account is in sandbox mode,")
		t.Logf("   you can only send to verified email addresses.")
		t.Logf("   Verify %s in SES Console or request production access.", testEmail)
		t.Logf("2. Check AWS credentials are configured correctly")
		t.Logf("3. Check spam folder")
		t.Logf("4. Verify the 'fromEmail' domain is verified in SES")
		require.NoError(t, err)
		return
	}

	t.Logf("✓ Email sent successfully to %s", testEmail)
	t.Logf("")
	t.Logf("NOTE: If you don't receive the email:")
	t.Logf("1. Check your spam/junk folder")
	t.Logf("2. Verify SES is not in sandbox mode (check AWS SES Console)")
	t.Logf("3. If in sandbox mode, verify %s in SES or request production access", testEmail)
	t.Logf("4. Check CloudWatch logs for delivery status")
}

// Note: Template rendering and preview functionality has been moved to EmailService.
// See internal/service/email.service_test.go for template preview tests.
