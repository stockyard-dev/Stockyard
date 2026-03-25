package platform

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	bidserver "github.com/stockyard-dev/stockyard/internal/bid/server"
	doubtserver "github.com/stockyard-dev/stockyard/internal/doubt/server"
	doubtstore "github.com/stockyard-dev/stockyard/internal/doubt/store"
	doubttracker "github.com/stockyard-dev/stockyard/internal/doubt/tracker"
	echohistory "github.com/stockyard-dev/stockyard/internal/echo/history"
	echoserver "github.com/stockyard-dev/stockyard/internal/echo/server"
	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
	faultmemory "github.com/stockyard-dev/stockyard/internal/fault/memory"
	faultserver "github.com/stockyard-dev/stockyard/internal/fault/server"
	faultstore "github.com/stockyard-dev/stockyard/internal/fault/store"
	fossilserver "github.com/stockyard-dev/stockyard/internal/fossil/server"
	fossilstore "github.com/stockyard-dev/stockyard/internal/fossil/store"
	grainaudit "github.com/stockyard-dev/stockyard/internal/grain/audit"
	grainserver "github.com/stockyard-dev/stockyard/internal/grain/server"
	grainstore "github.com/stockyard-dev/stockyard/internal/grain/store"
	graintree "github.com/stockyard-dev/stockyard/internal/grain/tree"
	hollowserver "github.com/stockyard-dev/stockyard/internal/hollow/server"
	hollowstore "github.com/stockyard-dev/stockyard/internal/hollow/store"
	ironserver "github.com/stockyard-dev/stockyard/internal/iron/server"
	ironstore "github.com/stockyard-dev/stockyard/internal/iron/store"
	morphserver "github.com/stockyard-dev/stockyard/internal/morph/server"
	morphstore "github.com/stockyard-dev/stockyard/internal/morph/store"
	orchestratorserver "github.com/stockyard-dev/stockyard/internal/orchestrator/server"
	prismmodel "github.com/stockyard-dev/stockyard/internal/prism/model"
	prismserver "github.com/stockyard-dev/stockyard/internal/prism/server"
	prismstore "github.com/stockyard-dev/stockyard/internal/prism/store"
	"github.com/stockyard-dev/stockyard/internal/provider"
	replayserver "github.com/stockyard-dev/stockyard/internal/replay/server"
	replaystore "github.com/stockyard-dev/stockyard/internal/replay/store"
	seanceserver "github.com/stockyard-dev/stockyard/internal/seance/server"
	spineserver "github.com/stockyard-dev/stockyard/internal/spine/server"
	spinestore "github.com/stockyard-dev/stockyard/internal/spine/store"
	stampedeserver "github.com/stockyard-dev/stockyard/internal/stampede/server"
	stampedestore "github.com/stockyard-dev/stockyard/internal/stampede/store"
	tideserver "github.com/stockyard-dev/stockyard/internal/tide/server"
	tidestore "github.com/stockyard-dev/stockyard/internal/tide/store"
	trailheadcstore "github.com/stockyard-dev/stockyard/internal/trailhead/cstore"
	trailheadserver "github.com/stockyard-dev/stockyard/internal/trailhead/server"
	verdiktserver "github.com/stockyard-dev/stockyard/internal/verdikt/server"

	relicserver "github.com/stockyard-dev/stockyard/internal/relic/server"
	relicstore "github.com/stockyard-dev/stockyard/internal/relic/store"
	breedserver "github.com/stockyard-dev/stockyard/internal/breed/server"
	breedstore "github.com/stockyard-dev/stockyard/internal/breed/store"
	fossilrecserver "github.com/stockyard-dev/stockyard/internal/fossilrec/server"
	fossilrecstore "github.com/stockyard-dev/stockyard/internal/fossilrec/store"
	phantomserver "github.com/stockyard-dev/stockyard/internal/phantom/server"
	phantomstore "github.com/stockyard-dev/stockyard/internal/phantom/store"
	feralserver "github.com/stockyard-dev/stockyard/internal/feral/server"
	feralstore "github.com/stockyard-dev/stockyard/internal/feral/store"
	tidepoolserver "github.com/stockyard-dev/stockyard/internal/tidepool/server"
	tidepoolstore "github.com/stockyard-dev/stockyard/internal/tidepool/store"
	crucibleserver "github.com/stockyard-dev/stockyard/internal/crucible/server"
	cruciblestore "github.com/stockyard-dev/stockyard/internal/crucible/store"
	cortexserver "github.com/stockyard-dev/stockyard/internal/cortex/server"
	cortexstore "github.com/stockyard-dev/stockyard/internal/cortex/store"
	myceliumserver "github.com/stockyard-dev/stockyard/internal/mycelium/server"
	myceliumstore "github.com/stockyard-dev/stockyard/internal/mycelium/store"
	sporeserver "github.com/stockyard-dev/stockyard/internal/spore/server"
	sporestore "github.com/stockyard-dev/stockyard/internal/spore/store"
	moltserver "github.com/stockyard-dev/stockyard/internal/molt/server"
	moltstore "github.com/stockyard-dev/stockyard/internal/molt/store"

	"github.com/stockyard-dev/stockyard/internal/orchestrator/brain"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/hub"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/workflow"
)

// ServerFactory holds shared resources for creating product servers.
type ServerFactory struct {
	DataDir   string
	DB        *sql.DB
	Providers map[string]provider.Provider
	EventBus  *bus.Bus
	Hub       *hub.Hub
}

// BuildAll creates all product servers that the given tier permits.
// Products above the tier are not instantiated (saves memory).
func (f *ServerFactory) BuildAll(tier Tier) []ProductServer {
	var servers []ProductServer

	type builder struct {
		id   string
		tier Tier
		fn   func() http.Handler
	}

	builders := []builder{
		// Individual
		{"bid", TierIndividual, f.buildBid},
		{"replay", TierIndividual, f.buildReplay},
		{"doubt", TierIndividual, f.buildDoubt},
		{"verdikt", TierIndividual, f.buildVerdikt},

		// Pro
		{"stampede", TierPro, f.buildStampede},
		{"fault", TierPro, f.buildFault},
		{"morph", TierPro, f.buildMorph},
		{"spine", TierPro, f.buildSpine},
		{"hollow", TierPro, f.buildHollow},

		// Team
		{"seance", TierTeam, f.buildSeance},
		{"grain", TierTeam, f.buildGrain},
		{"echo", TierTeam, f.buildEcho},
		{"tide", TierTeam, f.buildTide},
		{"fossil", TierTeam, f.buildFossil},
		{"prism", TierTeam, f.buildPrism},
		{"trailhead", TierTeam, f.buildTrailhead},

		// Enterprise
		{"iron", TierEnterprise, f.buildIron},
		{"orchestrator", TierEnterprise, f.buildOrchestrator},

		// ── New Products ──
		// Individual
		{"relic", TierIndividual, f.buildRelic},
		// Pro
		{"breed", TierPro, f.buildBreed},
		{"fossilrec", TierPro, f.buildFossilRec},
		// Team
		{"phantom", TierTeam, f.buildPhantom},
		{"feral", TierTeam, f.buildFeral},
		{"tidepool", TierTeam, f.buildTidePool},
		{"crucible", TierTeam, f.buildCrucible},
		// Enterprise
		{"cortex", TierEnterprise, f.buildCortex},
		{"mycelium", TierEnterprise, f.buildMycelium},
		{"spore", TierEnterprise, f.buildSpore},
		{"molt", TierEnterprise, f.buildMolt},
	}

	for _, b := range builders {
		if tier >= b.tier {
			handler := f.safeBuild(b.id, b.fn)
			if handler != nil {
				servers = append(servers, ProductServer{ID: b.id, Handler: handler})
			}
		}
	}

	return servers
}

// safeBuild catches panics during product initialization.
func (f *ServerFactory) safeBuild(id string, fn func() http.Handler) (handler http.Handler) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[platform] PANIC building %s: %v (product disabled)", id, r)
			handler = nil
		}
	}()
	return fn()
}

// productDir ensures a product data directory exists and returns it.
func (f *ServerFactory) productDir(name string) string {
	dir := filepath.Join(f.DataDir, name)
	os.MkdirAll(dir, 0o755)
	return dir
}

// firstProvider returns any available LLM provider, preferring openai.
func (f *ServerFactory) firstProvider() provider.Provider {
	if f.Providers == nil {
		return nil
	}
	for _, name := range []string{"openai", "anthropic", "gemini", "groq"} {
		if p, ok := f.Providers[name]; ok {
			return p
		}
	}
	for _, p := range f.Providers {
		return p
	}
	return nil
}

// ── Individual tier products ────────────────────────────────────────

func (f *ServerFactory) buildBid() http.Handler {
	return bidserver.New(bidserver.Config{})
}

func (f *ServerFactory) buildReplay() http.Handler {
	dir := f.productDir("replay")
	db, err := replaystore.Open(filepath.Join(dir, "replay.db"))
	if err != nil {
		log.Printf("[platform] replay: db open failed: %v", err)
		db = nil
	}
	return replayserver.New(replayserver.Config{DB: db})
}

func (f *ServerFactory) buildDoubt() http.Handler {
	dir := f.productDir("doubt")
	os.Setenv("DOUBT_DATA_DIR", dir)
	db, _ := doubtstore.Open(filepath.Join(dir, "doubt.db"))
	var ts *doubttracker.Store
	if db != nil {
		ts = doubttracker.New(db)
	}
	return doubtserver.New(doubtserver.Config{
		Dir:     dir,
		Tracker: ts,
	})
}

func (f *ServerFactory) buildVerdikt() http.Handler {
	return verdiktserver.New(verdiktserver.Config{})
}

// ── Pro tier products ───────────────────────────────────────────────

func (f *ServerFactory) buildStampede() http.Handler {
	dir := f.productDir("stampede")
	db, err := stampedestore.Open(filepath.Join(dir, "stampede.db"))
	if err != nil {
		log.Printf("[platform] stampede: db open failed: %v", err)
	}
	return stampedeserver.New(stampedeserver.Config{DB: db})
}

func (f *ServerFactory) buildFault() http.Handler {
	dir := f.productDir("fault")
	st, err := faultstore.Open(filepath.Join(dir, "fault.db"))
	if err != nil {
		log.Printf("[platform] fault: db open failed: %v", err)
	}
	mon := detect.New()
	diag := diagnose.New(f.firstProvider(), "")
	mem := faultmemory.New(st)
	return faultserver.New(faultserver.Config{
		Monitor:   mon,
		Diagnoser: diag,
		Store:     st,
		Memory:    mem,
	})
}

func (f *ServerFactory) buildMorph() http.Handler {
	dir := f.productDir("morph")
	st, _ := morphstore.New(filepath.Join(dir, "morph.db"))
	return morphserver.New(morphserver.Config{Store: st})
}

func (f *ServerFactory) buildSpine() http.Handler {
	dir := f.productDir("spine")
	st, _ := spinestore.New(filepath.Join(dir, "spine.db"))
	return spineserver.New(spineserver.Config{Store: st})
}

func (f *ServerFactory) buildHollow() http.Handler {
	dir := f.productDir("hollow")
	st, _ := hollowstore.New(filepath.Join(dir, "hollow.db"))
	return hollowserver.New(hollowserver.Config{Store: st})
}

// ── Team tier products ──────────────────────────────────────────────

func (f *ServerFactory) buildSeance() http.Handler {
	dir := f.productDir("seance")
	os.Setenv("SEANCE_DATA_DIR", dir)
	return seanceserver.New(seanceserver.Config{
		LLM: f.firstProvider(),
	})
}

func (f *ServerFactory) buildGrain() http.Handler {
	dir := f.productDir("grain")
	st, _ := grainstore.Open(filepath.Join(dir, "grain.db"))
	reg := graintree.NewRegistry(st)
	auditLog := grainaudit.NewLog(10000, st)
	return grainserver.New(grainserver.Config{
		Registry: reg,
		Audit:    auditLog,
		Store:    st,
	})
}

func (f *ServerFactory) buildEcho() http.Handler {
	dir := f.productDir("echo")
	db, _ := echohistory.Open(filepath.Join(dir, "echo.db"))
	return echoserver.New(echoserver.Config{DB: db})
}

func (f *ServerFactory) buildTide() http.Handler {
	dir := f.productDir("tide")
	st, _ := tidestore.Open(filepath.Join(dir, "tide.db"))
	return tideserver.New(tideserver.Config{Store: st})
}

func (f *ServerFactory) buildFossil() http.Handler {
	dir := f.productDir("fossil")
	st, _ := fossilstore.Open(filepath.Join(dir, "fossil.db"))
	return fossilserver.New(fossilserver.Config{Store: st})
}

func (f *ServerFactory) buildPrism() http.Handler {
	dir := f.productDir("prism")
	st, _ := prismstore.Open(filepath.Join(dir, "prism.db"))
	eng := prismmodel.New(st)
	return prismserver.New(prismserver.Config{
		Engine: eng,
		Store:  st,
	})
}

func (f *ServerFactory) buildTrailhead() http.Handler {
	dir := f.productDir("trailhead")
	db, _ := trailheadcstore.Open(filepath.Join(dir, "trailhead.db"))
	return trailheadserver.New(trailheadserver.Config{DB: db})
}

// ── Enterprise tier products ────────────────────────────────────────

func (f *ServerFactory) buildIron() http.Handler {
	dir := f.productDir("iron")
	st, err := ironstore.Open(filepath.Join(dir, "iron.db"))
	if err != nil {
		log.Printf("[platform] iron: db open failed: %v", err)
		return nil
	}
	return ironserver.New(st, ironserver.Config{DataDir: dir})
}

func (f *ServerFactory) buildOrchestrator() http.Handler {
	h := f.Hub
	if h == nil {
		h = hub.New()
		hub.RegisterAll(h)
	}
	b := f.EventBus
	if b == nil {
		b = bus.New()
	}
	wf := workflow.New(b, h)
	workflow.RegisterDefaults(wf)
	br := brain.New(f.firstProvider(), "", h, b)

	return orchestratorserver.New(orchestratorserver.Config{
		Hub:      h,
		Bus:      b,
		Workflow: wf,
		Brain:    br,
	})
}

// ── New product builders ────────────────────────────────────────────

func (f *ServerFactory) buildRelic() http.Handler {
	dir := f.productDir("relic")
	st, _ := relicstore.Open(filepath.Join(dir, "relic.db"))
	return relicserver.New(relicserver.Config{Store: st})
}

func (f *ServerFactory) buildBreed() http.Handler {
	dir := f.productDir("breed")
	st, _ := breedstore.Open(filepath.Join(dir, "breed.db"))
	return breedserver.New(breedserver.Config{Store: st})
}

func (f *ServerFactory) buildFossilRec() http.Handler {
	dir := f.productDir("fossilrec")
	st, _ := fossilrecstore.Open(filepath.Join(dir, "fossilrec.db"))
	return fossilrecserver.New(fossilrecserver.Config{Store: st})
}

func (f *ServerFactory) buildPhantom() http.Handler {
	dir := f.productDir("phantom")
	st, _ := phantomstore.Open(filepath.Join(dir, "phantom.db"))
	return phantomserver.New(phantomserver.Config{Store: st})
}

func (f *ServerFactory) buildFeral() http.Handler {
	dir := f.productDir("feral")
	st, _ := feralstore.Open(filepath.Join(dir, "feral.db"))
	return feralserver.New(feralserver.Config{Store: st})
}

func (f *ServerFactory) buildTidePool() http.Handler {
	dir := f.productDir("tidepool")
	st, _ := tidepoolstore.Open(filepath.Join(dir, "tidepool.db"))
	return tidepoolserver.New(tidepoolserver.Config{Store: st})
}

func (f *ServerFactory) buildCrucible() http.Handler {
	dir := f.productDir("crucible")
	st, _ := cruciblestore.Open(filepath.Join(dir, "crucible.db"))
	return crucibleserver.New(crucibleserver.Config{Store: st})
}

func (f *ServerFactory) buildCortex() http.Handler {
	dir := f.productDir("cortex")
	st, _ := cortexstore.Open(filepath.Join(dir, "cortex.db"))
	return cortexserver.New(cortexserver.Config{Store: st})
}

func (f *ServerFactory) buildMycelium() http.Handler {
	dir := f.productDir("mycelium")
	st, _ := myceliumstore.Open(filepath.Join(dir, "mycelium.db"))
	return myceliumserver.New(myceliumserver.Config{Store: st})
}

func (f *ServerFactory) buildSpore() http.Handler {
	dir := f.productDir("spore")
	st, _ := sporestore.Open(filepath.Join(dir, "spore.db"))
	return sporeserver.New(sporeserver.Config{Store: st})
}

func (f *ServerFactory) buildMolt() http.Handler {
	dir := f.productDir("molt")
	st, _ := moltstore.Open(filepath.Join(dir, "molt.db"))
	return moltserver.New(moltserver.Config{Store: st})
}
