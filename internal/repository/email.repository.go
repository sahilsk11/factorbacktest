package repository

// EmailRepository is responsible for sending emails.
// It's a wrapper around the email sending infrastructure
// that can send emails with templates.
type EmailRepository interface {
	// SendEmail sends an email to the specified recipient
	// with the given subject and body (HTML or plain text)
	SendEmail(to string, subject string, body string) error

	// SendEmailWithTemplate sends an email using a template
	// templateName is the name of the template to use
	// templateData is the data to populate the template
	SendEmailWithTemplate(to string, subject string, templateName string, templateData interface{}) error
}

type emailRepositoryHandler struct {
	// TODO: Add email provider client (e.g., AWS SES, SendGrid, etc.)
}

func NewEmailRepository() EmailRepository {
	return &emailRepositoryHandler{}
}

func (h *emailRepositoryHandler) SendEmail(to string, subject string, body string) error {
	// TODO: Implement email sending logic
	return nil
}

func (h *emailRepositoryHandler) SendEmailWithTemplate(to string, subject string, templateName string, templateData interface{}) error {
	// TODO: Implement template rendering and email sending logic
	return nil
}
