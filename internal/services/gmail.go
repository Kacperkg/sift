package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kacperkg/sift/internal/models"
)

type gmailList struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}
type gmailMessage struct {
	ID           string `json:"id"`
	ThreadID     string `json:"threadId"`
	InternalDate string `json:"internalDate"`
	Snippet      string `json:"snippet"`
	Payload      struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}

var (
	alertNoise = []string{
		"job alert", "jobs you may be interested", "more new jobs", "and other jobs",
		"you may be a fit", "is hiring:", "is hiring a", "hiring now:", "recruiter reacted",
		"recommended jobs", "new roles for you",
	}
	promotionNoise = []string{
		"% off", "sale", "discount", "save ", "free delivery", "free click", "deal",
		"reward", "prize draw", "shop now", "newsletter",
	}
	jobSignals = []string{
		"your application", "application received", "received your application",
		"received your job application", "thank you for applying", "thank you for your application",
		"application was sent", "application has been submitted", "application submitted",
		"candidate application", "your candidacy", "application update", "application status",
		"interview invitation", "invite you to interview", "schedule an interview", "schedule your interview",
		"selected for an interview", "offer you an interview", "next round", "next stage", "technical interview", "phone interview",
		"video interview", "screening call", "technical screen", "schedule a call",
		"not moving forward", "not to progress your application", "not progressing your application",
		"not taking your application further", "other candidates", "position has been filled", "will not proceed",
		"offer of employment", "employment offer", "offer letter", "pleased to offer you",
		"would like to offer you", "delighted to offer you", "extend an offer",
	}
	employmentOfferSignals = []string{
		"offer of employment", "employment offer", "formal offer", "offer letter",
		"pleased to offer you", "would like to offer you", "delighted to offer you",
		"happy to offer you", "extend an offer", "extending you an offer",
	}
	interviewSignals = []string{
		"interview invitation", "invite you to interview", "inviting you to interview",
		"schedule an interview", "schedule your interview", "interview availability",
		"available for an interview", "selected for an interview", "offer you an interview", "move forward with an interview",
		"proceed with an interview", "next round", "next stage", "technical interview",
		"phone interview", "video interview", "screening call", "technical screen",
		"schedule a call", "book a call", "meet the hiring team", "meet with the hiring team",
	}
	declineSignals = []string{
		"not moving forward", "not be moving forward", "decided not to progress",
		"not to progress your application", "not progress your application", "not progressing your application",
		"will not be progressing", "won't be progressing", "will not proceed",
		"not proceed with your application", "unable to progress", "not taking your application further",
		"not be taking your application further", "application was unsuccessful", "application has been unsuccessful",
		"unsuccessful on this occasion", "regret to inform you", "other candidates", "not selected",
		"position has been filled", "role has been filled", "vacancy has been filled",
	}
	senderNoise      = regexp.MustCompile(`(?i)\b(no[ -]?reply|do[- ]?not[- ]?reply|recruitment|recruiting|talent acquisition|talent|acquisition team|human resources|hr department|careers?|jobs?|notifications?|workdayhr|workday|greenhouse|department|mail)\b`)
	nonCompanySender = regexp.MustCompile(`(?i)^(alerts?|linkedin|indeed|greenhouse|lever|workday|ashby|teamtailor|smartrecruiters|jobgether|hiring team|team)$`)
)

func scanGmail(ctx context.Context, token string, ai *LocalAI, after time.Time, report func(models.SyncStatus)) ([]models.Application, error) {
	phase := "Searching Gmail"
	if !after.IsZero() {
		phase = "Checking for new email"
	}
	reportScanProgress(report, phase, 0, 0)
	query := gmailSearchQuery(after)
	endpoint := "https://gmail.googleapis.com/gmail/v1/users/me/messages?maxResults=300&q=" + url.QueryEscape(query)
	var list gmailList
	if err := gmailGet(ctx, token, endpoint, &list); err != nil {
		return nil, err
	}
	messages := make([]gmailMessage, len(list.Messages))
	loaded := make([]bool, len(list.Messages))
	reportScanProgress(report, "Reading email metadata", 0, len(list.Messages))
	semaphore := make(chan struct{}, 3)
	var waitGroup sync.WaitGroup
	var readCount atomic.Int64
	for index, item := range list.Messages {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			u := "https://gmail.googleapis.com/gmail/v1/users/me/messages/" + url.PathEscape(item.ID) + "?format=metadata&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Date"
			if err := gmailGet(ctx, token, u, &messages[index]); err == nil {
				loaded[index] = true
			}
			done := int(readCount.Add(1))
			reportScanProgress(report, "Reading email metadata", done, len(list.Messages))
		}()
	}
	waitGroup.Wait()

	apps := make([]models.Application, 0, len(list.Messages))
	reportScanProgress(report, "Classifying applications", 0, len(messages))
	for index, m := range messages {
		reportScanProgress(report, "Classifying applications", index+1, len(messages))
		if !loaded[index] {
			continue
		}
		h := map[string]string{}
		for _, x := range m.Payload.Headers {
			h[strings.ToLower(x.Name)] = x.Value
		}
		updated := time.Now()
		if milliseconds, err := strconv.ParseInt(m.InternalDate, 10, 64); err == nil {
			updated = time.UnixMilli(milliseconds)
		}
		company, role, status, ok := classify(h["from"], h["subject"], m.Snippet)
		staleApplied := status == "Applied" && updated.Before(time.Now().AddDate(0, 0, -14))
		potentialOutcome := shouldReviewPotentialOutcome(h["subject"] + " " + m.Snippet)
		if ai != nil && ai.IsEnabled() && (shouldAskLocalAI(h["subject"], company, role, ok) || (staleApplied && potentialOutcome)) {
			if result, err := ai.classify(ctx, h["from"], h["subject"], m.Snippet); err == nil && result.Confidence >= 0.75 {
				if !result.IsApplication {
					continue
				}
				ok = true
				if result.Company != "" {
					company = result.Company
				}
				if result.Role != "" {
					role = result.Role
				}
				status = result.Status
			}
		}
		if ok && company == "" && role == "" {
			ok = false
		}
		if !ok {
			continue
		}
		if company == "" {
			company = "Unknown company"
		}
		if role == "" {
			role = "Role not detected"
		}
		if status == "Applied" && updated.Before(time.Now().AddDate(0, 0, -14)) {
			status = "No response"
		}
		apps = append(apps, models.Application{ID: m.ID, ThreadID: m.ThreadID, Company: company, Role: role, Status: status, LastEvent: h["subject"], UpdatedAt: updated.Format(time.RFC3339)})
	}
	return mergeApplications(nil, apps), nil
}

func gmailSearchQuery(after time.Time) string {
	window := "newer_than:2y"
	if !after.IsZero() {
		window = "after:" + strconv.FormatInt(after.Unix(), 10)
	}
	return window + ` -category:promotions -category:social ("your application" OR "application received" OR "received your application" OR "thank you for applying" OR "thank you for your application" OR "application was sent" OR "job application" OR candidacy OR interview OR "not moving forward" OR "position has been filled" OR "offer of employment" OR "employment offer" OR "offer letter")`
}

// mergeApplications turns a stream of matching emails into application records.
// Gmail thread identity is authoritative. As a fallback, an email whose role
// could not be extracted is attached to the nearest same-company application
// within 14 days. Different detected roles at the same employer remain separate.
func mergeApplications(existing, incoming []models.Application) []models.Application {
	combined := append(append(make([]models.Application, 0, len(existing)+len(incoming)), existing...), incoming...)
	sort.SliceStable(combined, func(i, j int) bool {
		return applicationTime(combined[i]).Before(applicationTime(combined[j]))
	})

	merged := make([]models.Application, 0, len(combined))
	for _, candidate := range combined {
		match := -1
		bestDistance := 15 * 24 * time.Hour
		for index := range merged {
			if sameApplication(merged[index], candidate) {
				distance := applicationTime(merged[index]).Sub(applicationTime(candidate))
				if distance < 0 {
					distance = -distance
				}
				if match == -1 || distance < bestDistance {
					match = index
					bestDistance = distance
				}
			}
		}
		if match == -1 {
			merged = append(merged, candidate)
			continue
		}
		merged[match] = mergeApplicationRecord(merged[match], candidate)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return applicationTime(merged[i]).After(applicationTime(merged[j]))
	})
	return merged
}

func sameApplication(left, right models.Application) bool {
	if left.ThreadID != "" && right.ThreadID != "" && left.ThreadID == right.ThreadID {
		return true
	}
	if normalizeCompany(left.Company) != normalizeCompany(right.Company) {
		return false
	}
	leftRole, rightRole := normalizeRole(left.Role), normalizeRole(right.Role)
	if leftRole != normalizeRole("Role not detected") && rightRole != normalizeRole("Role not detected") {
		return leftRole == rightRole
	}
	distance := applicationTime(left).Sub(applicationTime(right))
	if distance < 0 {
		distance = -distance
	}
	return distance <= 14*24*time.Hour
}

func mergeApplicationRecord(older, newer models.Application) models.Application {
	if applicationTime(newer).Before(applicationTime(older)) {
		older, newer = newer, older
	}
	if newer.Role == "" || newer.Role == "Role not detected" {
		newer.Role = older.Role
	}
	if newer.Company == "" || newer.Company == "Unknown company" {
		newer.Company = older.Company
	}
	if newer.ThreadID == "" {
		newer.ThreadID = older.ThreadID
	}
	if older.Closed && !newer.Closed {
		newer.Status = older.Status
		newer.Closed = true
	}
	return newer
}

func carryClosedOutcomes(existing, scanned []models.Application) []models.Application {
	for index := range scanned {
		for _, previous := range existing {
			if !previous.Closed || !sameApplication(previous, scanned[index]) {
				continue
			}
			scanned[index].Status = previous.Status
			scanned[index].Closed = true
			break
		}
	}
	return scanned
}

func applicationTime(app models.Application) time.Time {
	parsed, err := time.Parse(time.RFC3339, app.UpdatedAt)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func normalizeCompany(company string) string {
	return strings.ToLower(strings.Join(strings.Fields(company), " "))
}

func reportScanProgress(report func(models.SyncStatus), phase string, processed, total int) {
	if report != nil {
		report(models.SyncStatus{Phase: phase, Processed: processed, Total: total})
	}
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.Join(strings.Fields(role), " "))
}

func shouldAskLocalAI(subject, company, role string, rulesAccepted bool) bool {
	lowerSubject := strings.ToLower(subject)
	if containsAny(lowerSubject, alertNoise...) || containsAny(lowerSubject, promotionNoise...) {
		return false
	}
	if !rulesAccepted {
		return true
	}
	return company == "Unknown company" || role == "Role not detected" || isProviderCompany(company)
}

func shouldReviewPotentialOutcome(text string) bool {
	text = strings.ToLower(text)
	return containsAny(text,
		"decid", "progress", "proceed", "forward", "further", "selected",
		"successful", "unsuccessful", "regret", "filled", "closed", "vacancy",
	)
}

func gmailGet(ctx context.Context, token, endpoint string, out any) error {
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusOK {
			err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
			resp.Body.Close()
			return err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 350 * time.Millisecond):
				continue
			}
		}
		return fmt.Errorf("Gmail API returned %d: %s", resp.StatusCode, body)
	}
	return errors.New("Gmail API retry limit reached")
}

func classify(from, subject, snippet string) (company, role, status string, ok bool) {
	lowerSubject := strings.ToLower(subject)
	text := strings.ToLower(subject + " " + snippet)
	if containsAny(lowerSubject, alertNoise...) || !containsAny(text, jobSignals...) {
		return "", "", "", false
	}
	if containsAny(lowerSubject, promotionNoise...) && !containsAny(lowerSubject, jobSignals...) {
		return "", "", "", false
	}

	status = "Applied"
	switch {
	case isDeclined(text):
		status = "Declined"
	case isEmploymentOffer(text):
		status = "Offer"
	case isInterviewInvitation(text):
		status = "Interviewing"
	}

	company = extractCompany(subject)
	if company == "" {
		company = extractCompany(snippet)
	}
	if company == "" {
		company = companyFromSender(from)
	}
	if company == "" {
		company = "Unknown company"
	}

	role = extractRole(subject)
	if role == "" {
		role = extractRole(snippet)
	}
	if parsedRole, parsedCompany := splitRoleAndCompany(role); parsedCompany != "" {
		role, company = parsedRole, parsedCompany
	}
	if strings.EqualFold(role, company) {
		role = ""
	}
	companySuffix := " at " + strings.ToLower(company)
	if strings.HasSuffix(strings.ToLower(role), companySuffix) {
		role = strings.TrimSpace(role[:len(role)-len(companySuffix)])
	}
	if role == "" {
		role = "Role not detected"
	}
	if role == "Role not detected" && isProviderCompany(company) {
		return "", "", "", false
	}
	if company == "Unknown company" && role == "Role not detected" {
		return "", "", "", false
	}
	return company, role, status, true
}

func isDeclined(text string) bool {
	return containsAny(text, declineSignals...)
}

func isEmploymentOffer(text string) bool {
	// Job boards routinely call a vacancy a "job offer". That phrase alone says
	// nothing about the candidate receiving an offer, so require decision language.
	if !containsAny(text, employmentOfferSignals...) {
		return false
	}
	return !containsAny(text,
		"offer you an interview",
		"offer you a chance to interview",
		"offer you the opportunity to interview",
	)
}

func isInterviewInvitation(text string) bool {
	if !containsAny(text, interviewSignals...) {
		return false
	}
	return !containsAny(text,
		"interview tips",
		"interview coaching",
		"how to interview",
		"next step is to reach out",
		"strongest next step",
	)
}

func extractCompany(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)see how\s+(.+?)\s+will assess your cv`),
		regexp.MustCompile(`(?i)application was sent to\s+([^.!|]+)`),
		regexp.MustCompile(`(?i)application at\s+(.+?)\s*(?:[-–—]|was received|received successfully|$)`),
		regexp.MustCompile(`(?i)thank you for your application to\s+([^.!|]+)`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(text); len(match) > 1 {
			return cleanCompany(match[1])
		}
	}
	return ""
}

func companyFromSender(from string) string {
	name := strings.TrimSpace(strings.Split(from, "<")[0])
	address := ""
	if parsed, err := mail.ParseAddress(from); err == nil {
		name, address = parsed.Name, parsed.Address
	}
	company := cleanCompany(senderNoise.ReplaceAllString(name, ""))
	if company != "" && !nonCompanySender.MatchString(company) && !strings.Contains(company, "@") {
		return company
	}
	parts := strings.Split(address, "@")
	if len(parts) != 2 {
		return ""
	}
	labels := strings.Split(strings.ToLower(parts[1]), ".")
	ignored := map[string]bool{"com": true, "co": true, "uk": true, "io": true, "net": true, "org": true, "test": true, "mail": true, "email": true, "jobs": true, "careers": true, "notifications": true, "linkedin": true, "indeed": true, "greenhouse": true, "lever": true, "workday": true, "myworkday": true, "myworkdayjobs": true, "ashbyhq": true}
	for i := len(labels) - 1; i >= 0; i-- {
		if ignored[labels[i]] || len(labels[i]) <= 2 || containsAny(labels[i], "greenhouse", "workday", "smartrecruit", "teamtailor") {
			continue
		}
		return titleToken(labels[i])
	}
	return ""
}

func cleanCompany(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSpace(strings.Trim(value, `"'<>:-–—,`))
	value = regexp.MustCompile(`(?i)\s+(group|team)$`).ReplaceAllString(value, " $1")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func extractRole(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)application (?:has been )?submitted to (?:the )?position,?\s*(.+?)(?:,\s*[A-Z0-9-]{8,}.*)?$`),
		regexp.MustCompile(`(?i)candidate application for\s+(.+)$`),
		regexp.MustCompile(`(?i)application received:\s*(.+?)(?:\s+[–—]\s+.*)?$`),
		regexp.MustCompile(`(?i)in regards to your application for (?:the )?(.+?)(?:\s+position)?$`),
		regexp.MustCompile(`(?i)your application\s+[A-Z]-?\d+\s*-\s*(.+?)\s+update$`),
		regexp.MustCompile(`(?i)your application\s+\d+\s+(.+?)(?:\s+\(open\))?$`),
		regexp.MustCompile(`(?i)application to\s+(.+?)\s+position confirmed$`),
		regexp.MustCompile(`(?i)your application to\s+(.+?)(?:\.\s|\s+[–—]\s|$)`),
		regexp.MustCompile(`(?i)application for (?:the )?(.+?)(?:\s+position)?$`),
		regexp.MustCompile(`(?i)thank you for applying (?:for|to)\s+(.+?)(?:\s+(?:at|with)\s+.+)?$`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(strings.TrimSpace(text)); len(match) > 1 {
			return cleanRole(match[1])
		}
	}
	return ""
}

func cleanRole(value string) string {
	value = html.UnescapeString(value)
	value = strings.TrimSpace(strings.Trim(value, `"'<>:-–—,.!`))
	if sentenceEnd := strings.Index(value, ". "); sentenceEnd >= 0 {
		value = value[:sentenceEnd]
	}
	value = regexp.MustCompile(`(?i)^(?:the )?position of\s+`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`(?i)^the\s+`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`(?i)\s+(?:application )?(?:status )?update$`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`(?i)\s+(?:role|position)$`).ReplaceAllString(value, "")
	value = strings.Join(strings.Fields(value), " ")
	lower := strings.ToLower(value)
	if len(value) > 120 || containsAny(lower, "your application", "application received", "thank you for applying", "successfully submitted", "we appreciate", "our team", "will review", "look forward", "your submission", "we're pleased", "we are pleased", "we're delighted", "we are delighted") {
		return ""
	}
	return value
}

func splitRoleAndCompany(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	separators := []string{" at ", " @ "}
	lower := strings.ToLower(value)
	for _, separator := range separators {
		if index := strings.LastIndex(lower, separator); index > 2 {
			role := cleanRole(value[:index])
			company := cleanCompany(value[index+len(separator):])
			if role != "" && company != "" && len(company) <= 80 && !strings.Contains(company, "http") {
				return role, company
			}
		}
	}
	return value, ""
}

func isProviderCompany(company string) bool {
	lower := strings.ToLower(strings.TrimSpace(company))
	providers := []string{"linkedin", "indeed", "wellfound", "workable", "greenhouse", "lever", "myworkday", "workday", "erecruiter", "traffit", "jobgether", "smartrecruiters", "teamtailor", "ashby"}
	for _, provider := range providers {
		if lower == provider || strings.HasPrefix(lower, provider+" ") {
			return true
		}
	}
	return false
}

func titleToken(value string) string {
	value = strings.ReplaceAll(value, "-", " ")
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
