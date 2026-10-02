package schema

// CreateApiKeyRequest is the admin's request to mint a new external-API key.
type CreateApiKeyRequest struct {
	Label string `json:"label" validate:"required,min=1,max=100"`
}

// CreateApiKeyResponse returns the raw key exactly once -- see
// service.ApiKeyService.Generate's doc comment; it is never retrievable
// again after this response, only its prefix/label/usage state via List.
type CreateApiKeyResponse struct {
	ID    uint   `json:"id"`
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ApiKeyResponse is one row of the key list -- never includes the raw key
// or its hash, only what's needed to recognize/audit it. LastUsedAt is 0
// (omitted) when the key has never authenticated a request yet.
type ApiKeyResponse struct {
	ID         uint   `json:"id"`
	Label      string `json:"label"`
	KeyPrefix  string `json:"key_prefix"`
	LastUsedAt int64  `json:"last_used_at,omitempty"`
	Revoked    bool   `json:"revoked"`
	CreatedAt  int64  `json:"created_at"`
}

type ListApiKeysResponse struct {
	Keys []ApiKeyResponse `json:"keys"`
}
