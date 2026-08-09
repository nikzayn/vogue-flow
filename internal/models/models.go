package models

// Represents user shopping intent
type Query struct {
	SessionID string            `json:"session_id"`
	UserID    string            `json:"user_id"`
	Text      string            `json:"text"`
	Embedding []float32         `json:"-"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Budget    float64           `json:"budget,omitempty"`
	Size      string            `json:"size,omitempty"`
	Ocassion  string            `json:"ocassion,omitempty"`
}

// Product which actually represnets the catalog item in Pinecone
type Product struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Price       float64           `json:"price"`
	Colors      []string          `json:"colors"`
	Sizes       []string          `json:"sizes"`
	InStock     bool              `json:"in_stock"`
	Score       float64           `json:"score"`
	Metadata    map[string]string `json:"metadata"`
}

type AgentState struct {
	Query Query `json:"query"`
}
