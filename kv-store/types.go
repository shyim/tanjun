package kv_store

type KVInput struct {
	Operation string `json:"operation"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
}

type KVResponse struct {
	Type         string `json:"type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Value        string `json:"value,omitempty"`
}
