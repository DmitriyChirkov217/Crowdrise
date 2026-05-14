package domain

import "math"

func ValidateSubmit(status, campaignType string, goalAmount float64, milestoneCount int, milestoneSum float64, rewardCount int) error {
	if status != ProjectDraft && status != ProjectRejected {
		return ErrInvalidStatus
	}
	if milestoneCount == 0 {
		return ErrValidation
	}
	if math.Abs(goalAmount-milestoneSum) > 0.009 {
		return ErrValidation
	}
	if campaignType == CampaignReward && rewardCount == 0 {
		return ErrValidation
	}
	if campaignType != CampaignReward && campaignType != CampaignDonation {
		return ErrValidation
	}
	return nil
}

func ApplyCapturedPayment(funds Funds, amount float64) (Funds, error) {
	if amount <= 0 {
		return funds, ErrValidation
	}
	funds.TotalCollected += amount
	funds.TotalReserved += amount
	return funds, nil
}

func ReleaseMilestoneFunds(funds Funds, amount float64) (Funds, error) {
	if amount <= 0 {
		return funds, ErrValidation
	}
	if funds.TotalReserved < amount {
		return funds, ErrInsufficientReservedFunds
	}
	funds.TotalReserved -= amount
	funds.TotalAvailable += amount
	return funds, nil
}

func RefundReservedFunds(funds Funds, amount float64) (Funds, error) {
	if amount <= 0 {
		return funds, ErrValidation
	}
	if funds.TotalReserved < amount {
		return funds, ErrInsufficientReservedFunds
	}
	funds.TotalReserved -= amount
	funds.TotalRefunded += amount
	return funds, nil
}
