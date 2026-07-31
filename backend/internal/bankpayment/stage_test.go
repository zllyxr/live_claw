package bankpayment

import "testing"

func TestBankPaymentStages(t *testing.T) {
	tests := []struct {
		status       int
		assigned     bool
		pendingProof bool
		want         string
	}{
		{status: 0, want: StageWaitingAssignment},
		{status: 1, assigned: true, want: StageAwaitingPayment},
		{status: 1, assigned: true, pendingProof: true, want: StageReviewPending},
		{status: 2, assigned: true, want: StagePaid},
		{status: 4, assigned: true, want: StageClosed},
	}
	for _, test := range tests {
		if got := Stage(test.status, test.assigned, test.pendingProof); got != test.want {
			t.Fatalf("Stage(%d,%t,%t)=%q want %q", test.status, test.assigned, test.pendingProof, got, test.want)
		}
	}
}
