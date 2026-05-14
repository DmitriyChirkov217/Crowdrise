package domain

import "testing"

func TestValidateSubmitRules(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		campaignType string
		goal         float64
		count        int
		sum          float64
		rewards      int
		wantErr      bool
	}{
		{"no milestones", ProjectDraft, CampaignDonation, 100, 0, 0, 0, true},
		{"wrong sum", ProjectDraft, CampaignDonation, 100, 1, 90, 0, true},
		{"reward needs reward", ProjectDraft, CampaignReward, 100, 1, 100, 0, true},
		{"donation ok without rewards", ProjectDraft, CampaignDonation, 100, 1, 100, 0, false},
		{"reward ok", ProjectDraft, CampaignReward, 100, 2, 100, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubmit(tt.status, tt.campaignType, tt.goal, tt.count, tt.sum, tt.rewards)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSubmit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFundsRules(t *testing.T) {
	funds, err := ApplyCapturedPayment(Funds{}, 1500)
	if err != nil {
		t.Fatal(err)
	}
	if funds.TotalCollected != 1500 || funds.TotalReserved != 1500 {
		t.Fatalf("captured payment mismatch: %+v", funds)
	}
	funds, err = ReleaseMilestoneFunds(funds, 500)
	if err != nil {
		t.Fatal(err)
	}
	if funds.TotalReserved != 1000 || funds.TotalAvailable != 500 {
		t.Fatalf("release mismatch: %+v", funds)
	}
	funds, err = RefundReservedFunds(funds, 600)
	if err != nil {
		t.Fatal(err)
	}
	if funds.TotalReserved != 400 || funds.TotalRefunded != 600 {
		t.Fatalf("refund mismatch: %+v", funds)
	}
	if _, err := ReleaseMilestoneFunds(funds, 500); err != ErrInsufficientReservedFunds {
		t.Fatalf("expected insufficient funds, got %v", err)
	}
}
