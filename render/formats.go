package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Grace's tick label formats, measured from gracebat
// (docs/grace-behaviour.md, "Tick label formats").

// formatNumber is a value in one of the numeric formats.
func formatNumber(v float64, format string, prec int) string {
	p := max(prec, 0)
	switch format {
	case "general", "":
		return fmt.Sprintf("%.*g", max(p, 1), v)
	case "exponential":
		return fmt.Sprintf("%.*e", p, v)
	case "power":
		// 10 to the value's decimal logarithm, at the precision
		switch {
		case v > 0:
			return fmt.Sprintf(`10\S%.*f\N`, p, math.Log10(v))
		case v < 0:
			return fmt.Sprintf(`-10\S%.*f\N`, p, math.Log10(-v))
		}
		return decimal(v, p)
	case "scientific":
		// the mantissa as rounded (999.99 at 1 is "10.0"), times 10 to the
		// exponent; 0 alone
		if v == 0 {
			return decimal(v, p)
		}
		e := math.Floor(math.Log10(math.Abs(v)))
		return fmt.Sprintf(`%.*f\x`+"\xb4"+`\f{}10\S%d\N`, p, v/math.Pow(10, e), int(e))
	case "engineering":
		// an SI prefix every 3 decades, y to Y, after a space
		e := 0
		if v != 0 {
			e = int(math.Floor(math.Log10(math.Abs(v))/3)) * 3
		}
		e = min(max(e, -24), 24)
		prefix := map[int]string{-24: "y", -21: "z", -18: "a", -15: "f", -12: "p", -9: "n", -6: `\xm\f{}`, -3: "m",
			0: "", 3: "k", 6: "M", 9: "G", 12: "T", 15: "P", 18: "E", 21: "Z", 24: "Y"}[e]
		return fmt.Sprintf("%.*f %s", p, v/math.Pow(10, float64(e)), prefix)
	case "computing":
		// from 1024 on, a binary prefix (K, M, ... Y), no space
		prefixes := []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
		k := 0
		for math.Abs(v) >= 1024 && k < len(prefixes)-1 {
			v /= 1024
			k++
		}
		return fmt.Sprintf("%.*g%s", max(p, 1), v, prefixes[k])
	}
	return decimal(v, p)
}

// decimal is v with p decimals; a value that rounds to zero has no sign.
func decimal(v float64, p int) string {
	s := strconv.FormatFloat(v, 'f', p, 64)
	if strings.HasPrefix(s, "-") && strings.Trim(s[1:], "0.") == "" {
		s = s[1:]
	}
	return s
}

var (
	monthsShort = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	monthsLong  = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	daysShort   = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	daysLong    = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

// civil is the calendar date of civil day z (the day starting at Julian
// day z - 0.5): the Julian calendar before 1582-10-15, the Gregorian from
// then; years astronomical (year 0 is 1 BC).
func civil(z int) (year, month, day int) {
	shift := 0
	if z < 0 { // the Julian calendar repeats every 1461 days
		k := (-z)/1461 + 1
		z += k * 1461
		shift = 4 * k
	}
	a := z
	if z >= 2299161 {
		alpha := int(math.Floor((float64(z) - 1867216.25) / 36524.25))
		a = z + 1 + alpha - alpha/4
	}
	b := a + 1524
	c := int(math.Floor((float64(b) - 122.1) / 365.25))
	d := int(math.Floor(365.25 * float64(c)))
	e := int(math.Floor(float64(b-d) / 30.6001))
	day = b - d - int(math.Floor(30.6001*float64(e)))
	month = e - 1
	if e >= 14 {
		month = e - 13
	}
	year = c - 4716
	if month <= 2 {
		year = c - 4715
	}
	return year - shift, month, day
}

// yearText: years before 1 AD are printed as BC years, negative (no year
// 0): Julian day 0 is in -4713.
func yearText(y int) string {
	if y <= 0 {
		y--
	}
	return strconv.Itoa(y)
}

// formatDate is a value in one of the date formats: days since the
// reference date, which counts from Julian day 0 (noon, 1 January 4713 BC).
func formatDate(v, ref float64, format string) (string, bool) {
	jd := v + ref
	if math.IsNaN(jd) || math.IsInf(jd, 0) || math.Abs(jd) > 1e9 {
		return "", false
	}
	exact := int(math.Floor(jd + 0.5)) // the civil day
	frac := jd + 0.5 - float64(exact)
	secs := int(math.Round(frac * 86400))
	if secs >= 86400 {
		secs -= 86400
		exact++
	}
	hms := fmt.Sprintf("%02d:%02d:%02d", secs/3600, secs/60%60, secs%60)
	rounded := int(math.Floor(jd + 1)) // to the nearest day, noon up
	y, m, d := civil(rounded)
	ey, em, ed := civil(exact)
	switch format {
	case "ddmmyy":
		return fmt.Sprintf("%02d-%02d-%s", d, m, yearText(y)), true
	case "mmddyy":
		return fmt.Sprintf("%02d-%02d-%s", m, d, yearText(y)), true
	case "yymmdd":
		return fmt.Sprintf("%s-%02d-%02d", yearText(y), m, d), true
	case "mmdd":
		return fmt.Sprintf("%02d-%02d", m, d), true
	case "monthday":
		return fmt.Sprintf("%s-%02d", monthsShort[m-1], d), true
	case "daymonth":
		return fmt.Sprintf("%02d-%s", d, monthsShort[m-1]), true
	case "dayofyear":
		first := rounded - (d - 1)
		for {
			if py, _, _ := civil(first - 1); py != y {
				break
			}
			first--
		}
		return strconv.Itoa(rounded - first + 1), true
	case "mmyy", "months", "monthsy", "monthl":
		// to the nearest month's start
		my, mm, md := civil(int(math.Floor(jd + 0.5)))
		start := int(math.Floor(jd+0.5)) - (md - 1)
		next := start + 28
		for {
			if _, nm, _ := civil(next); nm != mm {
				break
			}
			next++
		}
		t := jd + 0.5
		if t-float64(start) >= float64(next)-t {
			my, mm, _ = civil(next)
		}
		switch format {
		case "mmyy":
			return fmt.Sprintf("%02d-%s", mm, yearText(my)), true
		case "months":
			return monthsShort[mm-1], true
		case "monthsy":
			return fmt.Sprintf("%s-%s", monthsShort[mm-1], yearText(my)), true
		}
		return monthsLong[mm-1], true
	case "dayofweeks", "dayofweekl":
		w := ((int(math.Floor(jd+0.5))+1)%7 + 7) % 7
		if format == "dayofweeks" {
			return daysShort[w], true
		}
		return daysLong[w], true
	case "hms":
		return hms, true
	case "mmddhms":
		return fmt.Sprintf("%02d-%02d %s", em, ed, hms), true
	case "mmddyyhms":
		return fmt.Sprintf("%02d-%02d-%s %s", em, ed, yearText(ey), hms), true
	case "yymmddhms":
		return fmt.Sprintf("%s-%02d-%02d %s", yearText(ey), em, ed, hms), true
	}
	return "", false
}

// formatAngle is a value in one of the degree formats: degrees, minutes,
// seconds - the last at the precision - and E/W or N/S; 0 alone without.
func formatAngle(v float64, format string, prec int) (string, bool) {
	p := max(prec, 0)
	lat := strings.HasSuffix(format, "lat")
	kind := strings.TrimSuffix(strings.TrimSuffix(format, "lon"), "lat")
	if kind == format {
		return "", false
	}
	hemi := "E"
	switch {
	case lat && v < 0:
		hemi = "S"
	case lat:
		hemi = "N"
	case v < 0:
		hemi = "W"
	}
	a := math.Abs(v)
	deg := math.Floor(a)
	mins := (a - deg) * 60
	m := math.Floor(mins)
	sec := (mins - m) * 60
	switch kind {
	case "degrees":
		if v == 0 {
			return "0", true
		}
		return fmt.Sprintf("%.*f%s", p, a, hemi), true
	case "degreesmm":
		if v == 0 {
			return "0 0'", true
		}
		return fmt.Sprintf("%d %.*f' %s", int(deg), p, mins, hemi), true
	case "degreesmmss":
		if v == 0 {
			return `0 0' 0"`, true
		}
		return fmt.Sprintf(`%d %d' %.*f" %s`, int(deg), int(m), p, sec, hemi), true
	case "mmss":
		if v == 0 {
			return `0 0' 0"`, true
		}
		return fmt.Sprintf(`%d' %.*f" %s`, int(m), p, sec, hemi), true
	}
	return "", false
}
