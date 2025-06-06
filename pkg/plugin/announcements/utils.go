package announcements

import (
	"fmt"
	"strings"

	"github.com/andygrunwald/go-jira"
)

func determineClusterProduct(productID string, isHCP bool) (productName string) {
	if productID == "rosa" && isHCP {
		productName = "Red Hat OpenShift on AWS with Hosted Control Planes"
	} else if productID == "rosa" {
		productName = "Red Hat OpenShift on AWS"
	} else if productID == "osd" {
		productName = "OpenShift Dedicated"
	}
	return productName
}

func hasProduct(items []interface{}, target string) bool {
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if val, ok := m["value"].(string); ok && val == target {
				return true
			}
		}
	}
	return false
}

func buildJQL(projectKey string, filters []fieldQuery) string {
	var conditions []string
	for _, q := range filters {
		if q.Operator == "~*" {
			values := strings.Split(q.Value, ",")
			var orParts []string
			for _, v := range values {
				orParts = append(orParts,
					fmt.Sprintf(`(project = "%s" AND "%s" ~ "%s")`, projectKey, q.Field, strings.TrimSpace(v)))
			}
			conditions = append(conditions, "("+strings.Join(orParts, " OR ")+")")
		} else if q.Operator == "in" {
			conditions = append(conditions,
				fmt.Sprintf(`(project = "%s" AND "%s" in (%s))`, projectKey, q.Field, q.Value),
			)
		} else {
			conditions = append(conditions,
				fmt.Sprintf(`(project = "%s" AND "%s" %s "%s")`, projectKey, q.Field, q.Operator, q.Value),
			)
		}
	}
	return "(" + strings.Join(conditions, " OR ") + ") AND (labels is EMPTY OR labels not in (long-term))  AND status != Closed ORDER BY created DESC"
}

func formatVersion(version string) string {
	versionParts := strings.Split(version, ".")
	versionPrefix := version
	if len(versionParts) >= 2 {
		versionPrefix = fmt.Sprintf("%s.%s", versionParts[0], versionParts[1])
	}
	return versionPrefix
}

func isValidMatch(i jira.Issue, cluster clusterRecord) bool {
	isIgnored := func(val string) bool {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "none" || val == "n/a" || val == "all" || val == ""
	}

	hasMatchingValue := func(items []interface{}, expected string) bool {
		expected = strings.ToLower(strings.TrimSpace(expected))
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				if val, ok := m["value"].(string); ok {
					val = strings.ToLower(strings.TrimSpace(val))
					if val == expected {
						return true
					}
				}
			}
		}
		return false
	}

	productRaw := i.Fields.Unknowns[productCustomField]
	versionRaw := i.Fields.AffectsVersions
	nameRaw := i.Fields.Unknowns[customerNameCustomField]

	productMatch := false
	if items, ok := productRaw.([]interface{}); ok {
		productMatch = hasMatchingValue(items, cluster.Product)
	}
	if !productMatch {
		return false
	}

	versionMatch := false
	clusterFormattedVersion := formatVersion(cluster.Version)

	if versionRaw != nil {
		for _, v := range versionRaw {
			if v != nil {
				vFormatted := formatVersion(v.Name)
				if vFormatted == clusterFormattedVersion || isIgnored(v.Name) {
					versionMatch = true
					break
				}
			}
		}
	}

	nameMatch := false
	if nameStr, ok := nameRaw.(string); ok {
		parts := strings.Split(nameStr, ";")
		for _, part := range parts {
			val := strings.TrimSpace(part)
			if val == cluster.OrgName || isIgnored(val) {
				nameMatch = true
				break
			}
		}
	}

	return versionMatch || (nameMatch && versionMatch)
}
