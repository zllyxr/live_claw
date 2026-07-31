package bankpayment

const (
	StageWaitingAssignment = "waiting_assignment"
	StageAwaitingPayment   = "awaiting_payment"
	StageReviewPending     = "review_pending"
	StagePaid              = "paid"
	StageClosed            = "closed"
)

func Stage(orderStatus int, assigned bool, hasPendingProof bool) string {
	switch orderStatus {
	case 2:
		return StagePaid
	case 3, 4, 5:
		return StageClosed
	}
	if hasPendingProof {
		return StageReviewPending
	}
	if assigned {
		return StageAwaitingPayment
	}
	return StageWaitingAssignment
}
