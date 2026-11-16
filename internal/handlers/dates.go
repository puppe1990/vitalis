package handlers

// monthsPT is the Portuguese month list used by the diary header and the
// weekly range. Go's time package only formats English month names from a
// layout, and the app ships Portuguese copy, so the list lives here.
var monthsPT = [13]string{
	"", "janeiro", "fevereiro", "março", "abril", "maio", "junho",
	"julho", "agosto", "setembro", "outubro", "novembro", "dezembro",
}

// monthName returns the lowercase Portuguese month for a date.
func monthName(month int) string {
	if month < 1 || month > 12 {
		return ""
	}
	return monthsPT[month]
}

// longDatePT renders "16 de novembro" — the diary header's shape.
func longDatePT(dayOfMonth, month int) string {
	return itoa(int64(dayOfMonth)) + " de " + monthName(month)
}

// shortDatePT renders "16 nov", which fits the weekly range label.
func shortDatePT(dayOfMonth, month int) string {
	name := monthName(month)
	if len(name) > 3 {
		name = name[:3]
	}
	return itoa(int64(dayOfMonth)) + " " + name
}
