package aiusage

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

var errJWT = errors.New("invalid jwt")

type cursorUsageSummary struct {
	MembershipType  string `json:"membershipType"`
	IsUnlimited     bool   `json:"isUnlimited"`
	BillingCycleEnd string `json:"billingCycleEnd"`
	IndividualUsage *struct {
		Plan     *cursorPlanUsage `json:"plan"`
		OnDemand *cursorOnDemand  `json:"onDemand"`
	} `json:"individualUsage"`
	TeamUsage *struct {
		OnDemand *cursorOnDemand `json:"onDemand"`
	} `json:"teamUsage"`
	AutoModelSelectedDisplayMessage  string `json:"autoModelSelectedDisplayMessage"`
	NamedModelSelectedDisplayMessage string `json:"namedModelSelectedDisplayMessage"`
}

type cursorPlanUsage struct {
	AutoPercentUsed  *float64 `json:"autoPercentUsed"`
	APIPercentUsed   *float64 `json:"apiPercentUsed"`
	TotalPercentUsed *float64 `json:"totalPercentUsed"`
}

type cursorOnDemand struct {
	Enabled bool  `json:"enabled"`
	Used    int64 `json:"used"`
	Limit   int64 `json:"limit"`
}

func jwtSub(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", errJWT
	}
	payload, err := decodeJWTPart(parts[1])
	if err != nil {
		return "", errJWT
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Sub == "" {
		return "", errJWT
	}
	return claims.Sub, nil
}

func decodeJWTPart(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// parseCursorUsage maps GET /api/usage-summary JSON into limit bars.
// Unknown shapes return ok=false rather than 0% bars.
func parseCursorUsage(body []byte) (limits []domain.AIUsageLimit, details []domain.AIUsageDetail, ok bool) {
	var resp cursorUsageSummary
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, false
	}

	resetAt := formatResetAt(resp.BillingCycleEnd)
	plan := titleCaseWord(resp.MembershipType)

	if resp.IsUnlimited {
		if plan != "" {
			details = append(details, domain.AIUsageDetail{Label: "Plan", Value: plan, Depth: 0})
		}
		details = append(details, domain.AIUsageDetail{Label: "Unlimited", Depth: 0})
		return []domain.AIUsageLimit{}, details, true
	}

	autoPct, apiPct, totalPct, planLabel, parsed := cursorPoolPercents(resp)
	if !parsed {
		return nil, nil, false
	}
	if planLabel != "" {
		plan = planLabel
	}

	limits = []domain.AIUsageLimit{
		{Label: "Included", Percent: totalPct, ResetAt: resetAt},
		{Label: "Cursor Models", Percent: autoPct, ResetAt: resetAt},
		{Label: "Other Models", Percent: apiPct, ResetAt: resetAt},
	}
	if plan != "" {
		details = append(details, domain.AIUsageDetail{Label: "Plan", Value: plan, Depth: 0})
	}
	if d, ok := cursorOnDemandDetail(resp); ok {
		details = append(details, d)
	}
	return limits, details, true
}

func cursorPoolPercents(resp cursorUsageSummary) (auto, api, total int, plan string, ok bool) {
	if resp.IndividualUsage != nil && resp.IndividualUsage.Plan != nil {
		p := resp.IndividualUsage.Plan
		if p.AutoPercentUsed == nil || p.APIPercentUsed == nil || p.TotalPercentUsed == nil {
			return 0, 0, 0, "", false
		}
		a, aok := roundPct(*p.AutoPercentUsed)
		n, nok := roundPct(*p.APIPercentUsed)
		t, tok := roundPct(*p.TotalPercentUsed)
		if !aok || !nok || !tok {
			return 0, 0, 0, "", false
		}
		return a, n, t, "", true
	}

	autoRaw, autoOK := parsePercentFromMessage(resp.AutoModelSelectedDisplayMessage)
	apiRaw, apiOK := parsePercentFromMessage(resp.NamedModelSelectedDisplayMessage)
	if !autoOK || !apiOK {
		return 0, 0, 0, "", false
	}
	a, aok := roundPct(autoRaw)
	n, nok := roundPct(apiRaw)
	if !aok || !nok {
		return 0, 0, 0, "", false
	}
	total = a
	if n > total {
		total = n
	}
	plan = titleCaseWord(resp.MembershipType)
	if plan == "" {
		plan = "Cursor"
	}
	plan += " (team)"
	return a, n, total, plan, true
}

func cursorOnDemandDetail(resp cursorUsageSummary) (domain.AIUsageDetail, bool) {
	var od *cursorOnDemand
	if resp.IndividualUsage != nil {
		od = resp.IndividualUsage.OnDemand
	}
	if od == nil && resp.TeamUsage != nil {
		od = resp.TeamUsage.OnDemand
	}
	if od == nil || !od.Enabled {
		return domain.AIUsageDetail{}, false
	}
	if od.Used > 0 || od.Limit > 0 {
		return domain.AIUsageDetail{
			Label: "On-demand",
			Value: fmt.Sprintf("%s / %s", formatCents(od.Used), formatCents(od.Limit)),
			Depth: 0,
		}, true
	}
	return domain.AIUsageDetail{Label: "On-demand", Value: "on", Depth: 0}, true
}

func roundPct(v float64) (int, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	n := int(math.Round(v))
	if n < 0 {
		return 0, true
	}
	return n, true
}

func parsePercentFromMessage(msg string) (float64, bool) {
	i := strings.IndexByte(msg, '%')
	if i <= 0 {
		return 0, false
	}
	j := i - 1
	for j >= 0 && (msg[j] >= '0' && msg[j] <= '9' || msg[j] == '.') {
		j--
	}
	num := strings.TrimSpace(msg[j+1 : i])
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func formatResetAt(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, rfc3339)
	if err != nil {
		t, err = time.Parse(time.RFC3339, rfc3339)
		if err != nil {
			return ""
		}
	}
	return t.In(time.Local).Format("Jan 2 at 3:04pm")
}

func titleCaseWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func formatCents(cents int64) string {
	if cents < 0 {
		cents = 0
	}
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}
