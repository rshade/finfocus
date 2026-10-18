package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

const (
	envelopeSignature = "4dabf18193072939515e22adb298388d"
	plaintextMarker   = "PLAINTEXT-SECRET"
	credentialMarker  = "CREDENTIAL-VALUE"
)

type GenEmbedded struct {
	Shared engine.DataKeyed[any] `json:"shared,omitempty"`
}

type genLeaf struct {
	Name   string             `json:"name"`
	Props  map[string]any     `json:"props,omitempty"`
	Totals map[string]float64 `json:"totals,omitempty" datakey:"true"`
	Labels map[string]any     `json:"labels,omitempty" datakey:"true"`
}

type genRoot struct {
	GenEmbedded

	Leaf   *genLeaf              `json:"leaf"`
	Leaves []genLeaf             `json:"leaves"`
	Any    any                   `json:"any"`
	Keyed  engine.DataKeyed[any] `json:"keyed,omitempty"`
	ByName map[string]genLeaf    `json:"byName"`
	Props  map[string]any        `json:"props"`
	Quiet  string                `json:"-"`
}

func hiddenNames() []string {
	return []string{"password", "token", "apiKey", "__defaults", "client_secret"}
}

func dataNames() []string { return []string{"urn:web", "us-east-1", "2026-01-02", "compute"} }

func plainNames() []string {
	return []string{"name", "size", "tags", "deltas", "resource", "key", "property"}
}

func anyNames() []string { return append(hiddenNames(), dataNames()...) }

type valueGen struct {
	rnd     *rand.Rand
	counter int
}

func (g *valueGen) pick(names []string) string { return names[g.rnd.Intn(len(names))] }

func (g *valueGen) envelope() map[string]any {
	g.counter++
	return map[string]any{
		envelopeSignature: "1",
		"value":           fmt.Sprintf("%s-%d", plaintextMarker, g.counter),
	}
}

func (g *valueGen) scalar() any {
	g.counter++
	switch g.rnd.Intn(3) {
	case 0:
		return float64(g.counter)
	case 1:
		return g.counter%2 == 0
	default:
		return fmt.Sprintf("v%d", g.counter)
	}
}

func (g *valueGen) plainMap(depth int) map[string]any {
	out := map[string]any{}
	for range g.rnd.Intn(5) {
		switch g.rnd.Intn(4) {
		case 0:
			g.counter++
			out[g.pick(hiddenNames())] = fmt.Sprintf("%s-%d", credentialMarker, g.counter)
		case 1:
			out[g.pick(plainNames())] = g.envelope()
		default:
			out[g.pick(plainNames())+g.pick([]string{"", "2"})] = g.any(depth + 1)
		}
	}
	return out
}

func (g *valueGen) keyed(depth int) engine.DataKeyed[any] {
	if g.rnd.Intn(8) == 0 {
		return engine.DataKeyed[any]{envelopeSignature: "1", "value": plaintextMarker + "-keyed"}
	}
	out := engine.DataKeyed[any]{}
	for range g.rnd.Intn(5) {
		name := g.pick(anyNames())
		switch g.rnd.Intn(4) {
		case 0:
			out[name] = g.envelope()
		case 1:
			out[name] = g.any(depth + 1)
		default:
			out[name] = g.scalar()
		}
	}
	return out
}

func (g *valueGen) any(depth int) any {
	if depth > 3 {
		return g.scalar()
	}
	switch g.rnd.Intn(8) {
	case 6:
		return injectingSlice{{"urn:web": 1}}
	case 7:
		return injectingMap{"urn:web": 1}
	case 0:
		return g.plainMap(depth)
	case 1:
		return g.keyed(depth)
	case 2:
		list := make([]any, g.rnd.Intn(4))
		for i := range list {
			if g.rnd.Intn(4) == 0 {
				list[i] = g.envelope()
				continue
			}
			list[i] = g.any(depth + 1)
		}
		return list
	case 3:
		return g.envelope()
	default:
		return g.scalar()
	}
}

func (g *valueGen) leaf(depth int) genLeaf {
	g.counter++
	leaf := genLeaf{Name: fmt.Sprintf("leaf%d", g.counter)}
	if g.rnd.Intn(2) == 0 {
		leaf.Props = g.plainMap(depth)
	}
	if g.rnd.Intn(2) == 0 {
		leaf.Totals = map[string]float64{}
		for range g.rnd.Intn(4) {
			leaf.Totals[g.pick(anyNames())] = float64(g.counter)
		}
	}
	if g.rnd.Intn(2) == 0 {
		leaf.Labels = map[string]any{}
		for range g.rnd.Intn(4) {
			name := g.pick(anyNames())
			if g.rnd.Intn(3) == 0 {
				leaf.Labels[name] = g.envelope()
			} else {
				leaf.Labels[name] = g.any(depth + 1)
			}
		}
	}
	return leaf
}

func (g *valueGen) root() genRoot {
	root := genRoot{Any: g.any(0)}
	if g.rnd.Intn(2) == 0 {
		root.Shared = g.keyed(1)
	}
	if g.rnd.Intn(2) == 0 {
		leaf := g.leaf(1)
		root.Leaf = &leaf
	}
	for range g.rnd.Intn(4) {
		root.Leaves = append(root.Leaves, g.leaf(1))
	}
	if g.rnd.Intn(2) == 0 {
		root.Keyed = g.keyed(1)
	}
	if g.rnd.Intn(2) == 0 {
		root.ByName = map[string]genLeaf{g.pick(anyNames()): g.leaf(1)}
	}
	root.Props = g.plainMap(0)
	return root
}

func assertNoLeak(t *testing.T, seed int64, value any) {
	t.Helper()
	data, err := RedactJSON(value)
	require.NoError(t, err, "seed %d", seed)
	assert.NotContains(t, string(data), plaintextMarker, "seed %d leaked a secret value", seed)
	assert.NotContains(t, string(data), credentialMarker, "seed %d leaked a credential-named value", seed)
	assert.NotContains(t, string(data), envelopeSignature, "seed %d leaked an envelope", seed)
	assert.True(t, json.Valid(data), "seed %d", seed)
}

func TestRedactJSONRandomizedNeverLeaks(t *testing.T) {
	t.Parallel()

	for seed := range int64(3000) {
		g := &valueGen{rnd: rand.New(rand.NewSource(seed))}
		assertNoLeak(t, seed, g.root())
	}
}

func FuzzRedactJSONNeverLeaks(f *testing.F) {
	for _, seed := range []int64{0, 1, 7, 42, 1000} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed int64) {
		g := &valueGen{rnd: rand.New(rand.NewSource(seed))}
		assertNoLeak(t, seed, g.root())
	})
}

func TestRedactJSONKeepsHiddenNamesOnlyAsDataKeys(t *testing.T) {
	t.Parallel()

	type payload struct {
		Totals map[string]float64 `json:"totals" datakey:"true"`
		Props  map[string]any     `json:"props"`
		Keyed  map[string]any     `json:"keyed"  datakey:"true"`
	}
	data, err := RedactJSON(payload{
		Totals: map[string]float64{"password": 1, "urn:web": 2},
		Props:  map[string]any{"password": "p", "size": "m"},
		Keyed:  map[string]any{"token": map[string]any{"password": "nested", "size": "s"}, "ok": "fine"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"totals": {"password": 1, "urn:web": 2},
		"props": {"size": "m"},
		"keyed": {"token": {"size": "s"}, "ok": "fine"}
	}`, string(data))
}

func TestRedactJSONDropsSecretEnvelopeAtEveryDepth(t *testing.T) {
	t.Parallel()

	secret := func() map[string]any {
		return map[string]any{envelopeSignature: "1", "value": plaintextMarker}
	}
	type leaf struct {
		Labels map[string]any `json:"labels" datakey:"true"`
	}
	type payload struct {
		Nested leaf                  `json:"nested"`
		Items  []leaf                `json:"items"`
		Any    engine.DataKeyed[any] `json:"any"`
		Tagged map[string]any        `json:"tagged" datakey:"true"`
		Whole  map[string]any        `json:"whole"  datakey:"true"`
	}
	data, err := RedactJSON(payload{
		Nested: leaf{Labels: map[string]any{"a": secret(), "urn:web": "kept"}},
		Items:  []leaf{{Labels: map[string]any{"b": secret()}}, {Labels: map[string]any{"urn:web:2": 1}}},
		Any:    engine.DataKeyed[any]{"deep": map[string]any{"c": secret(), "n": 1}},
		Tagged: map[string]any{"d": secret(), "list": []any{secret(), "x"}},
		Whole:  secret(),
	})
	require.NoError(t, err)
	assert.NotContains(t, string(data), plaintextMarker)
	assert.JSONEq(t, `{
		"nested": {"labels": {"urn:web": "kept"}},
		"items": [{"labels": {}}, {"labels": {"urn:web:2": 1}}],
		"any": {"deep": {"n": 1}},
		"tagged": {"list": ["x"]}
	}`, string(data))
}

func TestRedactJSONPromotesFieldsOfUnexportedEmbeddedStruct(t *testing.T) {
	t.Parallel()

	type base struct {
		Totals map[string]float64 `json:"totals" datakey:"true"`
	}
	type payload struct {
		base
	}
	data, err := RedactJSON(payload{base{Totals: map[string]float64{"password": 1}}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"totals": {"password": 1}}`, string(data))
}

func TestRedactJSONNilAndEmptyShapes(t *testing.T) {
	t.Parallel()

	type payload struct {
		Nil    map[string]float64    `json:"nilMap"         datakey:"true"`
		Empty  engine.DataKeyed[int] `json:"empty"`
		Omit   map[string]float64    `json:"omit,omitempty" datakey:"true"`
		Ptr    *genLeaf              `json:"ptr"`
		Slice  []genLeaf             `json:"slice"`
		Blank  []genLeaf             `json:"blank"`
		Iface  any                   `json:"iface"`
		Hidden map[string]float64    `json:"token"          datakey:"true"`
	}
	data, err := RedactJSON(
		payload{Empty: engine.DataKeyed[int]{}, Blank: []genLeaf{}, Hidden: map[string]float64{"a": 1}},
	)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"nilMap": null, "empty": {}, "ptr": null, "slice": null, "blank": [], "iface": null}`,
		string(data),
	)
}

type injectingMap map[string]float64

func (injectingMap) MarshalJSON() ([]byte, error) {
	return []byte(`{"password":"` + credentialMarker + `","urn:web":1}`), nil
}

func TestRedactJSONTreatsStructEmbeddingDataKeyedAsPropertyMap(t *testing.T) {
	t.Parallel()

	type payload struct {
		engine.DataKeyed[any]

		Password string `json:"password"`
		Size     string `json:"size"`
	}
	data, err := RedactJSON(payload{Password: credentialMarker, Size: "m"})
	require.NoError(t, err)
	assert.NotContains(t, string(data), credentialMarker)
	assert.Contains(t, string(data), `"size":"m"`)
}

func TestRedactJSONDoesNotTrustTaggedMapWithOwnEncoding(t *testing.T) {
	t.Parallel()

	type payload struct {
		Totals injectingMap `json:"totals" datakey:"true"`
	}
	data, err := RedactJSON(payload{Totals: injectingMap{"urn:web": 1}})
	require.NoError(t, err)
	assert.NotContains(t, string(data), credentialMarker)
	assert.JSONEq(t, `{"totals": {"urn:web": 1}}`, string(data))
}

func TestRedactJSONPrefersTaggedFieldAmongEqualDepthEmbeddedNames(t *testing.T) {
	t.Parallel()

	type left struct {
		Totals map[string]float64 `json:"Totals" datakey:"true"`
	}
	type right struct {
		Totals map[string]float64
	}
	type payload struct {
		left
		right
	}
	data, err := RedactJSON(payload{left: left{Totals: map[string]float64{"password": 1}}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"Totals": {"password": 1}}`, string(data))

	type bare struct {
		Totals map[string]float64
	}
	type other struct {
		Totals map[string]float64
	}
	type clash struct {
		bare
		other
	}
	clashed, err := RedactJSON(clash{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(clashed))
}

func TestRedactValueRedactsGoTypedContainers(t *testing.T) {
	t.Parallel()

	got := RedactValue(map[string]any{
		"a":    map[string]string{"password": credentialMarker, "size": "m"},
		"b":    []map[string]any{{"token": credentialMarker, "size": "s"}},
		"c":    genLeaf{Name: "x", Props: map[string]any{"apiKey": credentialMarker}},
		"keep": engine.DataKeyed[string]{"password": "ok"},
	})
	data, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(data), credentialMarker)
	assert.JSONEq(t, `{
		"a": {"size": "m"},
		"b": [{"size": "s"}],
		"c": {"name": "x", "props": {}},
		"keep": {"password": "ok"}
	}`, string(data))
}

func TestRedactJSONWhileSessionStateChanges(t *testing.T) {
	t.Parallel()

	session := NewSession(context.Background(), SessionOptions{DayOfMonth: 10})
	row := func(n int) engine.OverviewRow {
		return engine.OverviewRow{
			URN:        fmt.Sprintf("urn:pulumi:dev::demo::aws:ec2/instance:Instance::web-%d", n),
			Type:       "aws:ec2/instance:Instance",
			Status:     engine.StatusActive,
			Properties: map[string]any{"password": credentialMarker, "size": "m"},
			ActualCost: &engine.ActualCostData{
				MTDCost: 1, Currency: "USD", Breakdown: map[string]float64{"password": 1},
			},
		}
	}
	rows := []engine.OverviewRow{row(0), row(1), row(2)}
	session.SetData(nil, rows, engine.DateRange{}, "dev")
	_, events, cancel := session.subscribe()
	defer cancel()
	go func() {
		for range events {
		}
	}()

	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				changed := row(worker)
				session.Row(i%3, changed)
				changed.Properties["size"] = fmt.Sprintf("l%d", i)
				changed.ActualCost.Breakdown["compute"] = float64(i)
				_, _, drop := session.subscribe()
				drop()
			}
		}()
	}
	wg.Wait()

	_, _, drop := session.subscribe()
	drop()
	session.mu.Lock()
	data, err := RedactJSON(session.snapshotLocked())
	session.mu.Unlock()
	require.NoError(t, err)
	assert.NotContains(t, string(data), credentialMarker)
	assert.Contains(t, string(data), `"password"`)
}

type injectingSlice []engine.DataKeyed[any]

func (injectingSlice) MarshalJSON() ([]byte, error) {
	return []byte(`[{"password":"` + credentialMarker + `","urn:web":1}]`), nil
}

func TestRedactJSONTreatsCustomEncodedDeltasLikeAnyOtherKey(t *testing.T) {
	t.Parallel()

	source := injectingSlice{{"urn:web": 1}}
	for _, key := range []string{"deltas", "items"} {
		data, err := RedactJSON(map[string]any{key: source})
		require.NoError(t, err)
		assert.NotContains(t, string(data), credentialMarker, key)
		assert.JSONEq(t, `{"`+key+`": [{"urn:web": 1}]}`, string(data))
	}
}
