package models

type BatchGPURequest struct {
	VastAICount int                    `json:"vastai_count"`
	IONetCount  int                    `json:"ionet_count"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

type BatchGPUResponse struct {
	VastAIInstances []GPUInstance `json:"vastai_instances"`
	IONetInstances  []GPUInstance `json:"ionet_instances"`
	TotalCreated    int           `json:"total_created"`
	Errors          []string      `json:"errors,omitempty"`
}
