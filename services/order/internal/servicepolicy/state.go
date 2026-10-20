package servicepolicy

func CanChangeStatus(from, to string) bool {
	switch from {
	case "created":
		return to == "ready_for_delivery" || to == "cancelled"
	case "ready_for_delivery":
		return to == "courier_assigned"
	case "courier_assigned":
		return to == "delivering" || to == "delivered"
	case "delivering":
		return to == "delivered"
	default:
		return false
	}
}
