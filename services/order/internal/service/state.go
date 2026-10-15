package service

func CanChangeStatus(from, to string) bool {
	switch from {
	case "created":
		return to == "confirmed" || to == "cancelled"

	case "confirmed":
		return to == "preparing" || to == "cancelled"

	case "preparing":
		return to == "ready_for_delivery" || to == "cancelled"

	case "ready_for_delivery":
		return to == "courier_assigned"

	case "courier_assigned":
		return to == "delivering"

	case "delivering":
		return to == "delivered"

	case "delivered":
		return false

	case "cancelled":
		return false

	default:
		return false
	}
}
