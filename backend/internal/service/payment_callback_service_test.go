package service

import "testing"

func TestEvaluateRechargeOrderStatusForPayment(t *testing.T) {
	tests := []struct {
		name             string
		status           string
		wantCanProcess   bool
		wantAlreadyPayed bool
	}{
		{
			name:             "pending can be processed",
			status:           OrderStatusPending,
			wantCanProcess:   true,
			wantAlreadyPayed: false,
		},
		{
			name:             "expired can be compensated",
			status:           OrderStatusExpired,
			wantCanProcess:   true,
			wantAlreadyPayed: false,
		},
		{
			name:             "failed can be compensated",
			status:           OrderStatusFailed,
			wantCanProcess:   true,
			wantAlreadyPayed: false,
		},
		{
			name:             "cancelled can be compensated",
			status:           OrderStatusCancelled,
			wantCanProcess:   true,
			wantAlreadyPayed: false,
		},
		{
			name:             "paid is already processed",
			status:           OrderStatusPaid,
			wantCanProcess:   false,
			wantAlreadyPayed: true,
		},
		{
			name:             "refunded is already processed",
			status:           "refunded",
			wantCanProcess:   false,
			wantAlreadyPayed: true,
		},
		{
			name:             "unknown status is rejected",
			status:           "unknown_status",
			wantCanProcess:   false,
			wantAlreadyPayed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canProcess, alreadyProcessed := evaluateRechargeOrderStatusForPayment(tt.status)

			if canProcess != tt.wantCanProcess {
				t.Fatalf("canProcess mismatch: got %v, want %v", canProcess, tt.wantCanProcess)
			}
			if alreadyProcessed != tt.wantAlreadyPayed {
				t.Fatalf("alreadyProcessed mismatch: got %v, want %v", alreadyProcessed, tt.wantAlreadyPayed)
			}
		})
	}
}
