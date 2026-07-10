package dto

type CreateIncidentRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
	Severity    string `json:"severity" binding:"required"`
}
