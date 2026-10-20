package services

import (
	"strings"
	"testing"
	"time"

	"github.com/Kacperkg/sift/internal/models"
)

func TestClassify(t *testing.T) {
	company, role, status, ok := classify("Acme Talent <jobs@acme.test>", "Your application for Software Engineer", "Thank you for applying")
	if !ok || company != "Acme" || role != "Software Engineer" || status != "Applied" {
		t.Fatalf("got %q %q %q %v", company, role, status, ok)
	}
	_, _, status, ok = classify("Acme Hiring <hiring@acme.test>", "Your application update", "Unfortunately, we are not moving forward")
	if !ok || status != "Declined" {
		t.Fatalf("got %q %v", status, ok)
	}
}

func TestLloydsNotProgressingIsDeclined(t *testing.T) {
	company, role, status, ok := classify(
		"Lloyds Banking Group Recruitment <noreply@lloydsbanking.com>",
		"Your Application 161725 Software Engineer (Open)",
		"Thanks for your interest in the 161725 Software Engineer (Open) role. We've decided not to progress your application further for this particular vacancy.",
	)
	if !ok || company != "Lloyds Banking Group" || role != "Software Engineer" || status != "Declined" {
		t.Fatalf("got company=%q role=%q status=%q ok=%v", company, role, status, ok)
	}
}

func TestUnfortunatelyAloneDoesNotMeanDeclined(t *testing.T) {
	_, _, status, ok := classify(
		"Acme Talent <jobs@acme.test>",
		"Your application for Software Engineer",
		"Unfortunately the review is taking longer than expected, but your application remains under consideration.",
	)
	if !ok || status != "Applied" {
		t.Fatalf("got status=%q ok=%v", status, ok)
	}
}

func TestOnlyDecisionLikeStaleMessagesNeedOutcomeReview(t *testing.T) {
	if !shouldReviewPotentialOutcome("We have decided not to take your application further") {
		t.Fatal("missed decision-like wording")
	}
	if shouldReviewPotentialOutcome("Thank you for applying. We received your application.") {
		t.Fatal("flagged an ordinary acknowledgement for AI outcome review")
	}
}

func TestJobListingOfferIsAppliedNotEmploymentOffer(t *testing.T) {
	company, role, status, ok := classify(
		"No Fluff Jobs <notifications@nofluffjobs.com>",
		"Your application for Junior React Developer @ Link Group",
		"Your application has been sent successfully. Job offer: Junior React Developer Company: Link Group",
	)
	if !ok || company != "Link Group" || role != "Junior React Developer" || status != "Applied" {
		t.Fatalf("got company=%q role=%q status=%q ok=%v", company, role, status, ok)
	}
}

func TestEmploymentOfferNeedsHiringDecisionLanguage(t *testing.T) {
	tests := []struct {
		snippet string
		want    string
	}{
		{"We are pleased to offer you the Software Engineer position.", "Offer"},
		{"Your formal offer letter is attached.", "Offer"},
		{"We would like to offer you an interview next week.", "Interviewing"},
	}
	for _, test := range tests {
		_, _, status, ok := classify("Acme Talent <jobs@acme.test>", "Your application for Software Engineer", test.snippet)
		if !ok || status != test.want {
			t.Errorf("%q: got status=%q ok=%v, want %q", test.snippet, status, ok, test.want)
		}
	}
}

func TestJobgetherAdviceIsAppliedNotInterviewing(t *testing.T) {
	company, role, status, ok := classify(
		"Jobgether <support@match.jobgether.com>",
		"Your application to Frontend Developer (React). See how Київстар will assess your CV",
		"You've just applied to Frontend Developer (React) at Київстар! The hiring decision now sits with Київстар. The strongest next step is to reach out to the hiring team.",
	)
	if !ok || company != "Київстар" || role != "Frontend Developer (React)" || status != "Applied" {
		t.Fatalf("got company=%q role=%q status=%q ok=%v", company, role, status, ok)
	}
}

func TestInterviewingRequiresAnActualInvitation(t *testing.T) {
	tests := []struct {
		subject, snippet, want string
	}{
		{"Your application update", "The strongest next step is to reach out to the hiring team.", "Applied"},
		{"Interview invitation", "We would like to invite you to interview for the role.", "Interviewing"},
		{"Your application update", "Please choose your availability to schedule your interview.", "Interviewing"},
	}
	for _, test := range tests {
		_, _, status, ok := classify("Acme Talent <jobs@acme.test>", test.subject, test.snippet)
		if !ok || status != test.want {
			t.Errorf("%q: got status=%q ok=%v, want %q", test.snippet, status, ok, test.want)
		}
	}
}

func TestRejectsPromotionsAndJobAlerts(t *testing.T) {
	for _, test := range []struct{ from, subject, snippet string }{
		{"Sports Direct <offers@sportsdirect.com>", "Up to 70% off Nike", "Limited offer"},
		{"eBay <deals@ebay.com>", "Save 20% today", "A special offer"},
		{"LinkedIn Alerts <jobalerts@linkedin.com>", "Acme is hiring: Software Engineer", "Job alert"},
	} {
		if _, _, _, ok := classify(test.from, test.subject, test.snippet); ok {
			t.Fatalf("accepted non-application email %q", test.subject)
		}
	}
}

func TestProviderSpecificExtraction(t *testing.T) {
	tests := []struct {
		from, subject, snippet, company, role string
	}{
		{"LinkedIn <jobs-noreply@linkedin.com>", "Kacper, your application was sent to Ladders", "Your application was sent", "Ladders", "Role not detected"},
		{"NO REPLY - BNP Paribas Recruitment <noreply@bnp.test>", "Your application has been submitted to the position, Junior Software Developer_Waw, 1234567890100123293", "", "BNP Paribas", "Junior Software Developer_Waw"},
		{"NatWest <recruitment@natwest.com>", "Your application R-00284098 - Software Engineer update", "", "NatWest", "Software Engineer"},
		{"WorkdayHR <noreply@company.test>", "Your Application 161725 Software Engineer (Open)", "", "Company", "Software Engineer"},
	}
	for _, test := range tests {
		company, role, _, ok := classify(test.from, test.subject, test.snippet)
		if !ok || company != test.company || role != test.role {
			t.Errorf("%q: got company=%q role=%q ok=%v", test.subject, company, role, ok)
		}
	}
}

func TestSplitsRoleAndEmployer(t *testing.T) {
	role, company := splitRoleAndCompany("Frontend Developer at Luxoft Poland")
	if role != "Frontend Developer" || company != "Luxoft Poland" {
		t.Fatalf("got role=%q company=%q", role, company)
	}
}

func TestDropsProviderOnlyRows(t *testing.T) {
	_, _, _, ok := classify("Wellfound <notifications@wellfound.com>", "Your application update", "Your application is being reviewed")
	if ok {
		t.Fatal("accepted provider-only row without employer or role")
	}
}

func TestMergeMissingRoleIntoNearestSameCompanyApplication(t *testing.T) {
	apps := mergeApplications(nil, []models.Application{
		{ID: "availability", Company: "Precisely", Role: "Associate Experience Engineer", Status: "Interviewing", LastEvent: "Precisely Interview Availability", UpdatedAt: "2026-09-15T09:00:00Z"},
		{ID: "confirmation", Company: "Precisely", Role: "Role not detected", Status: "Interviewing", LastEvent: "Precisely Interview Confirmation - Response Required", UpdatedAt: "2026-09-15T10:00:00Z"},
	})
	if len(apps) != 1 {
		t.Fatalf("got %d records, want 1", len(apps))
	}
	if apps[0].Role != "Associate Experience Engineer" || apps[0].ID != "confirmation" {
		t.Fatalf("got role=%q id=%q", apps[0].Role, apps[0].ID)
	}
}

func TestMergeKeepsDifferentDetectedRolesAtSameCompany(t *testing.T) {
	apps := mergeApplications(nil, []models.Application{
		{ID: "frontend", Company: "Acme", Role: "Frontend Engineer", Status: "Applied", UpdatedAt: "2026-09-15T09:00:00Z"},
		{ID: "backend", Company: "Acme", Role: "Backend Engineer", Status: "Applied", UpdatedAt: "2026-09-15T10:00:00Z"},
	})
	if len(apps) != 2 {
		t.Fatalf("got %d records, want 2", len(apps))
	}
}

func TestMergeUsesGmailThreadAndPreservesKnownRole(t *testing.T) {
	apps := mergeApplications(nil, []models.Application{
		{ID: "old", ThreadID: "thread-1", Company: "Acme", Role: "Software Engineer", Status: "Applied", UpdatedAt: "2026-09-01T09:00:00Z"},
		{ID: "new", ThreadID: "thread-1", Company: "Acme", Role: "Role not detected", Status: "Interviewing", UpdatedAt: "2026-09-20T09:00:00Z"},
	})
	if len(apps) != 1 || apps[0].Role != "Software Engineer" || apps[0].Status != "Interviewing" || apps[0].ID != "new" {
		t.Fatalf("unexpected merged record: %#v", apps)
	}
}

func TestGmailSearchQuerySupportsIncrementalUpdates(t *testing.T) {
	after := time.Unix(1_700_000_000, 0)
	query := gmailSearchQuery(after)
	if !strings.Contains(query, "after:1700000000") || strings.Contains(query, "newer_than:2y") {
		t.Fatalf("unexpected incremental query: %q", query)
	}
	if !strings.Contains(gmailSearchQuery(time.Time{}), "newer_than:2y") {
		t.Fatal("full scan query lost its two-year window")
	}
}

func TestClosedOutcomeSurvivesIncrementalUpdate(t *testing.T) {
	existing := []models.Application{{
		ID: "old", ThreadID: "thread-1", Company: "Acme", Role: "Software Engineer",
		Status: "Accepted", Closed: true, UpdatedAt: "2026-09-20T09:00:00Z",
	}}
	incoming := []models.Application{{
		ID: "new", ThreadID: "thread-1", Company: "Acme", Role: "Software Engineer",
		Status: "Interviewing", UpdatedAt: "2026-09-21T09:00:00Z",
	}}
	apps := mergeApplications(existing, incoming)
	if len(apps) != 1 || apps[0].Status != "Accepted" || !apps[0].Closed || apps[0].ID != "new" {
		t.Fatalf("manual outcome was not preserved: %#v", apps)
	}
}

func TestClosedOutcomeSurvivesFullRescan(t *testing.T) {
	existing := []models.Application{{
		ID: "old", ThreadID: "thread-1", Company: "Acme", Role: "Software Engineer",
		Status: "Declined", Closed: true, UpdatedAt: "2026-09-20T09:00:00Z",
	}}
	scanned := []models.Application{{
		ID: "new", ThreadID: "thread-1", Company: "Acme", Role: "Software Engineer",
		Status: "Interviewing", UpdatedAt: "2026-09-21T09:00:00Z",
	}}
	apps := carryClosedOutcomes(existing, scanned)
	if apps[0].Status != "Declined" || !apps[0].Closed {
		t.Fatalf("manual outcome was not preserved: %#v", apps)
	}
}
