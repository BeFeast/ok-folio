package gallery

import (
	"errors"
	"strings"
)

const (
	// CustomMVPDirection keeps the MVP gallery inside OK Folio instead of
	// mutating the live PhotoPrism runtime.
	CustomMVPDirection = "custom-gallery-mvp"
)

// Option compares an architecture path for the OK Folio gallery surface.
type Option struct {
	Name      string   `json:"name"`
	Strengths []string `json:"strengths"`
	Risks     []string `json:"risks"`
	MVPFit    string   `json:"mvp_fit"`
	Decision  string   `json:"decision"`
}

// StorageImplication records runtime and storage tradeoffs for the chosen path.
type StorageImplication struct {
	Topic       string `json:"topic"`
	Implication string `json:"implication"`
}

// VerificationStep gives operators a live check that does not deploy or mutate
// the current PhotoPrism installation.
type VerificationStep struct {
	Name     string `json:"name"`
	Command  string `json:"command,omitempty"`
	Expected string `json:"expected"`
}

// PrototypePlan describes the issue-scoped MVP prototype.
type PrototypePlan struct {
	Surface      string             `json:"surface"`
	DataPath     string             `json:"data_path"`
	LiveRoute    string             `json:"live_route"`
	Verification []VerificationStep `json:"verification"`
	NonGoals     []string           `json:"non_goals"`
}

// Decision is the current gallery architecture decision for the MVP.
type Decision struct {
	Product          string               `json:"product"`
	ChosenDirection  string               `json:"chosen_direction"`
	Summary          string               `json:"summary"`
	Options          []Option             `json:"options"`
	Storage          []StorageImplication `json:"storage"`
	Prototype        PrototypePlan        `json:"prototype"`
	RuntimeGuardrail []string             `json:"runtime_guardrail"`
}

// MVPDecision returns the current tested gallery decision and prototype path.
func MVPDecision() Decision {
	return Decision{
		Product:         "OK Folio",
		ChosenDirection: CustomMVPDirection,
		Summary: "Prototype a custom OK Folio gallery surface for the MVP, backed by the existing extractor database and local image endpoints. " +
			"Keep PhotoPrism as an optional legacy index target while the product validates provider-first gallery workflows.",
		Options: []Option{
			{
				Name: "Customize PhotoPrism",
				Strengths: []string{
					"Existing full-featured gallery, search, albums, thumbnails, and metadata handling.",
					"Already compatible with the legacy extractor indexing flow.",
				},
				Risks: []string{
					"Branding and workflow changes depend on upstream PhotoPrism extension points or fork maintenance.",
					"Provider connector UX would remain coupled to PhotoPrism's data model and release cadence.",
					"Changing the live runtime risks current gallery availability without a deploy runbook and rollback.",
				},
				MVPFit:   "Fastest if the MVP is only a photo browser, weaker for OK Folio provider orchestration.",
				Decision: "Do not customize for this MVP prototype.",
			},
			{
				Name: "Wrap PhotoPrism",
				Strengths: []string{
					"Preserves PhotoPrism as the heavy media engine while OK Folio owns navigation and provider status.",
					"Can be introduced as a reverse-proxy or companion UI without immediately migrating media.",
				},
				Risks: []string{
					"Still requires API/session integration and careful route ownership.",
					"Favorites and indexing semantics stay split between OK Folio and PhotoPrism.",
					"Live verification would need deploy and rollback discipline before touching LAN routes.",
				},
				MVPFit:   "Viable later if PhotoPrism remains the gallery engine, but too deployment-sensitive for this issue.",
				Decision: "Keep as a later integration option.",
			},
			{
				Name: "Custom OK Folio gallery",
				Strengths: []string{
					"Owns provider-first workflows, typed API contracts, and OK Folio identity.",
					"Uses the existing extractor database and image handlers for a small, testable MVP.",
					"Can run beside PhotoPrism without mutating its storage, database, or LAN route.",
				},
				Risks: []string{
					"Must build missing gallery features incrementally: durable thumbnails, albums, richer metadata, and background indexing.",
					"Needs explicit storage rules before remote filesystems or multiple gallery nodes are supported.",
				},
				MVPFit:   "Best match for validating OK Folio as a self-hosted provider gallery without live PhotoPrism changes.",
				Decision: "Choose for the MVP prototype.",
			},
		},
		Storage: []StorageImplication{
			{
				Topic:       "Storage locality",
				Implication: "Keep originals and derived assets on the same storage for the MVP so extractor writes, gallery reads, snapshots, and rollback stay local.",
			},
			{
				Topic:       "Network-storage deployment",
				Implication: "If the API runs away from the storage host over network storage, treat media files as shared read-mostly data, avoid rename-heavy thumbnail workflows, and keep database state authoritative.",
			},
			{
				Topic:       "PhotoPrism coexistence",
				Implication: "Do not write into PhotoPrism storage or trigger live indexing from the prototype unless a deploy runbook and rollback are ready.",
			},
		},
		Prototype: PrototypePlan{
			Surface:   "Custom gallery API metadata endpoint plus existing local image, search, today, week, artist, and thumbnail endpoints.",
			DataPath:  "Extractor database rows are the gallery catalog; file bytes are served from configured storage paths.",
			LiveRoute: "/api/v1/gallery/decision",
			Verification: []VerificationStep{
				{
					Name:     "Product verifier",
					Command:  "./scripts/product-verifier.sh",
					Expected: "Dashboard build, staged embed assets, and Go tests pass from the worktree.",
				},
				{
					Name:     "LAN route smoke test",
					Command:  "curl -fsS http://<lan-host>:<api-port>/api/v1/gallery/decision",
					Expected: "Returns chosen_direction=custom-gallery-mvp and does not require PhotoPrism credentials.",
				},
			},
			NonGoals: []string{
				"Deploying or changing platform-managed runtime services.",
				"Changing live PhotoPrism configuration, storage, database, or route ownership.",
				"Adding Immich support.",
			},
		},
		RuntimeGuardrail: []string{
			"This prototype is read-only with respect to PhotoPrism.",
			"Do not touch live PhotoPrism runtime until deploy and rollback runbooks exist.",
			"Keep provider credentials, runtime config, downloaded media, thumbnails, and database dumps out of git.",
		},
	}
}

// ValidateDecision verifies the decision still covers the issue acceptance criteria.
func ValidateDecision(d Decision) error {
	if d.Product != "OK Folio" {
		return errors.New("decision must use OK Folio product identity")
	}
	if d.ChosenDirection != CustomMVPDirection {
		return errors.New("MVP direction must be the custom gallery prototype")
	}
	if len(d.Options) < 3 {
		return errors.New("decision must compare PhotoPrism customization, wrapping, and custom gallery")
	}

	if len(d.Storage) == 0 {
		return errors.New("decision must document storage implications")
	}

	text := strings.ToLower(joinDecisionText(d))
	for _, required := range []string{"photoprism", "custom", "runbook", "rollback", d.Prototype.LiveRoute} {
		if !strings.Contains(text, strings.ToLower(required)) {
			return errors.New("decision is missing required topic: " + required)
		}
	}
	if len(d.Prototype.Verification) == 0 {
		return errors.New("decision must include a live verification path")
	}
	return nil
}

func joinDecisionText(d Decision) string {
	var b strings.Builder
	b.WriteString(d.Summary)
	b.WriteString(" ")
	b.WriteString(d.Prototype.Surface)
	b.WriteString(" ")
	b.WriteString(d.Prototype.DataPath)
	b.WriteString(" ")
	b.WriteString(d.Prototype.LiveRoute)
	for _, verification := range d.Prototype.Verification {
		b.WriteString(" ")
		b.WriteString(verification.Name)
		b.WriteString(" ")
		b.WriteString(verification.Command)
		b.WriteString(" ")
		b.WriteString(verification.Expected)
	}
	for _, nonGoal := range d.Prototype.NonGoals {
		b.WriteString(" ")
		b.WriteString(nonGoal)
	}
	for _, option := range d.Options {
		b.WriteString(" ")
		b.WriteString(option.Name)
		b.WriteString(" ")
		b.WriteString(option.MVPFit)
		b.WriteString(" ")
		b.WriteString(option.Decision)
		for _, value := range option.Strengths {
			b.WriteString(" ")
			b.WriteString(value)
		}
		for _, value := range option.Risks {
			b.WriteString(" ")
			b.WriteString(value)
		}
	}
	for _, storage := range d.Storage {
		b.WriteString(" ")
		b.WriteString(storage.Topic)
		b.WriteString(" ")
		b.WriteString(storage.Implication)
	}
	for _, guardrail := range d.RuntimeGuardrail {
		b.WriteString(" ")
		b.WriteString(guardrail)
	}
	return b.String()
}
