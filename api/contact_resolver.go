package api

import (
	"alpha/internal/db/models/postgres/public/model"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contactResponse map[string]string

type contactRequest struct {
	UserID     *string `json:"userID"`
	ReplyEmail *string `json:"replyEmail"`
	Content    string  `json:"content"`
}

func (h ApiHandler) contact(c *gin.Context) {
	tx, err := h.Db.Begin()
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	defer tx.Rollback()

	var requestBody contactRequest

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	if requestBody.ReplyEmail != nil && len(*requestBody.ReplyEmail) > 320 {
		err = fmt.Errorf("invalid email - too long")
	}
	if len(requestBody.Content) < 5 {
		err = fmt.Errorf("contact message too short - must be > 5 characters")
	}
	if len(requestBody.Content) > 2000 {
		err = fmt.Errorf("contact message too long - must be < 2000 characters")
	}
	if err != nil {
		returnErrorJsonCode(err, c, 400)
		return
	}

	in := model.ContactMessage{
		ReplyEmail:     requestBody.ReplyEmail,
		MessageContent: requestBody.Content,
	}

	if requestBody.UserID != nil {
		userID, err := uuid.Parse(*requestBody.UserID)
		if err == nil {
			in.UserID = &userID
		}
	}

	err = h.ContactRepository.Add(tx, in)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	err = tx.Commit()
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to commit transaction: %w", err), c)
		return
	}

	out := map[string]string{
		"message": "ok",
	}

	c.JSON(200, out)
}
