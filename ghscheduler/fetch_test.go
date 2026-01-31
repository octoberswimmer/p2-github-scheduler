package ghscheduler

import (
	"testing"
)

func TestExtractSchedulingStatus(t *testing.T) {
	tests := []struct {
		name       string
		issueData  map[string]any
		wantStatus string
	}{
		{
			name:       "empty_data",
			issueData:  map[string]any{},
			wantStatus: "",
		},
		{
			name: "on_hold_status",
			issueData: map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fieldValues": map[string]any{
										"nodes": []any{
											map[string]any{
												"field": map[string]any{
													"name": "Scheduling Status",
												},
												"name": "On Hold",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantStatus: "On Hold",
		},
		{
			name: "active_status",
			issueData: map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fieldValues": map[string]any{
										"nodes": []any{
											map[string]any{
												"field": map[string]any{
													"name": "Scheduling Status",
												},
												"name": "Active",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantStatus: "Active",
		},
		{
			name: "no_scheduling_status_field",
			issueData: map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fieldValues": map[string]any{
										"nodes": []any{
											map[string]any{
												"field": map[string]any{
													"name": "Other Field",
												},
												"name": "Some Value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSchedulingStatus(tt.issueData)
			if got != tt.wantStatus {
				t.Errorf("ExtractSchedulingStatus() = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}
