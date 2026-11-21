package genuikit_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

func plain(value any) any {
	switch v := value.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = plain(item)
		}
		return out
	case *openuilang.Object:
		out := map[string]any{}
		for _, key := range v.Keys {
			out[key] = plain(v.Values[key])
		}
		return out
	}
	return value
}

func library(t *testing.T) *openuilang.Library {
	t.Helper()
	zone := time.FixedZone("reader", -7*3600)
	draws := []float64{0.5, 0.99, 0.0, 0.25}
	next := 0
	lib, err := genuikit.NewLibrary(genuikit.Environment{
		Now:      func() time.Time { return time.Date(2026, 10, 10, 2, 30, 0, 0, time.UTC) },
		Location: zone,
		Random: func() float64 {
			next++
			return draws[(next-1)%len(draws)]
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func value(t *testing.T, lib *openuilang.Library, expression string) string {
	t.Helper()
	session := openuilang.NewSession()
	session.Load(lib.Parse("root = Col([" + expression + "])"))
	root, _ := session.Render()
	if root == nil {
		t.Fatalf("%s did not parse", expression)
	}
	children, _ := root.Props.Get("children")
	list, _ := children.([]any)
	if len(list) != 1 {
		return "null"
	}
	out, err := json.Marshal(plain(list[0]))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestFunctionsFollowTheirDescriptions(t *testing.T) {
	rows := `[{team: "a", n: 2, on: "2026-10-05"}, {team: "b", n: 5, on: "2026-10-07"}, {team: "a", n: 3, on: "2026-10-19"}]`
	cases := []struct{ expression, want string }{
		{`@Append([1, 2], 3)`, `[1,2,3]`},
		{`@Prepend([1, 2], 3)`, `[3,1,2]`},
		{`@Concat([1], [2, 3])`, `[1,2,3]`},
		{`@Remove(["a", "b", "a"], "", "a")`, `["b"]`},
		{`@Remove([{id: 1}, {id: 2}], "id", "2")`, `[{"id":1}]`},
		{`@RemoveAt([1, 2, 3], -1)`, `[1,2]`},
		{`@Update([{id: 1, done: false}, {id: 2, done: false}], "id", 2, "done", true)`, `[{"done":false,"id":1},{"done":true,"id":2}]`},
		{`@Toggle(["a", "b"], "a")`, `["b"]`},
		{`@Toggle(["a"], "b")`, `["a","b"]`},
		{`@Includes([1, 2], "2")`, `true`},
		{`@Includes("spaghetti", "ghet")`, `true`},
		{`@Includes([1, 2], 3)`, `false`},
		{`@Find([{id: 1, name: "x"}, {id: 2, name: "y"}], "id", 2).name`, `"y"`},
		{`@Find([{id: 1}], "id", 9)`, `null`},
		{`@IndexOf(["a", "b"], "b")`, `1`},
		{`@IndexOf(["a"], "z")`, `-1`},
		{`@Take([1, 2, 3], 2)`, `[1,2]`},
		{`@Take([1, 2, 3], -2)`, `[2,3]`},
		{`@Slice([1, 2, 3, 4], 1, -1)`, `[2,3]`},
		{`@Slice("lisbon", 0, 3)`, `"lis"`},
		{`@Slice("lisbon", -3)`, `"bon"`},
		{`@Unique([1, "1", 2, {a: 1}, {a: 1}])`, `[1,2,{"a":1}]`},
		{`@Reverse([1, 2, 3])`, `[3,2,1]`},
		{`@Range(0, 3)`, `[0,1,2]`},
		{`@Range(0, 1, 0.25)`, `[0,0.25,0.5,0.75]`},
		{`@Range(3, 0, -1)`, `[3,2,1]`},
		{`@Range(0, 3, 0)`, `null`},
		{`@Flatten([[1, 2], [3], 4])`, `[1,2,3,4]`},
		{`@Chunk([1, 2, 3, 4, 5], 2)`, `[[1,2],[3,4],[5]]`},
		{`@Chunk([1], 0)`, `null`},

		{`@Pow(2, 10)`, `1024`},
		{`@Pow(10, 400)`, `null`},
		{`@Sqrt(81)`, `9`},
		{`@Sqrt(-1)`, `null`},
		{`@Clamp(12, 1, 10)`, `10`},
		{`@Clamp(5, 10, 1)`, `5`},
		{`@Percent(3, 4)`, `75`},
		{`@Percent(3, 0)`, `0`},
		{`@Lerp(10, 20, 0.25)`, `12.5`},
		{`@Allocate(100, [1, 1, 1])`, `[33.34,33.33,33.33]`},
		{`@Allocate(10, [0, 0])`, `[0,0]`},
		{`@Allocate(-1, [1, 1], 0)`, `[-1,0]`},
		{`@Allocate(10, [1, -1])`, `null`},
		{`@Round(@Payment(300000, 6 / 12, 360), 2)`, `1798.65`},
		{`@Payment(1200, 0, 12)`, `100`},
		{`@Payment(1200, 1, 0)`, `null`},
		{`@Convert(1, "mi", "km")`, `1.609344`},
		{`@Convert(212, "F", "c")`, `100`},
		{`@Convert(1, "kg", "km")`, `null`},

		{`@Format(12345.6)`, `"12,345.6"`},
		{`@Format(12345.678, "currency")`, `"$12,345.68"`},
		{`@Format(-5, "currency", 0, "eur")`, `"-€5"`},
		{`@Format(12, "currency", null, "CHF")`, "\"CHF\u00a012.00\""},
		{`@Format(45, "percent")`, `"45%"`},
		{`@Format(12.345, "percent", 2)`, `"12.35%"`},
		{`@Format(1234567, "compact")`, `"1.2M"`},
		{`@Format(1200000, "compact", 1, "USD")`, `"$1.2M"`},
		{`@Format(999, "compact")`, `"999"`},
		{`@Format(999999, "compact")`, `"1M"`},
		{`@Format(1500, "compact")`, `"1.5K"`},
		{`@Format(2, "number", 2)`, `"2"`},
		{`@Join(["a", null, 2, {k: 1}])`, `"a, 2, {\"k\":1}"`},
		{`@Join([{n: "x"}, {n: "y"}], " / ", "n")`, `"x / y"`},
		{`@Plural(1, "guest")`, `"1 guest"`},
		{`@Plural(3, "guest")`, `"3 guests"`},
		{`@Plural(1200, "person", "people")`, `"1,200 people"`},
		{`@Pad(7, 2)`, `"07"`},
		{`@Pad("ab", 5, "xy")`, `"xyxab"`},
		{`@Upper("ok")`, `"OK"`},
		{`@Lower("OK")`, `"ok"`},
		{`@Trim("  a b ")`, `"a b"`},
		{`@Capitalize("new york-style (thin)")`, `"New York-Style (Thin)"`},
		{`@Split("a, b,,c")`, `["a","b","c"]`},
		{`@Split("a b", " ")`, `["a","b"]`},
		{`@Replace("a-b-c", "-", "+")`, `"a+b+c"`},
		{`@Replace("abc", "", "x")`, `"abc"`},
		{`@Duration(4800)`, `"1h 20m"`},
		{`@Duration(245)`, `"4m 05s"`},
		{`@Duration(45)`, `"45s"`},
		{`@Duration(3600)`, `"1h"`},
		{`@Duration(4800, "clock")`, `"1:20:00"`},
		{`@Duration(245, "clock")`, `"4:05"`},
		{`@Default(null, 0)`, `0`},
		{`@Default("", "none")`, `"none"`},
		{`@Default(0, 5)`, `0`},
		{`@MixColor("#ffffff", "#0285ff", 0.5)`, `"#81c2ff"`},
		{`@MixColor("fff", "#000", 2)`, `"#000000"`},
		{`@MixColor("red", "#000", 1)`, `null`},

		{`@Now()`, `"2026-10-10T02:30:00.000Z"`},
		{`@Today()`, `"2026-10-09"`},
		{`@FormatDate("2026-10-09")`, `"Oct 9, 2026"`},
		{`@FormatDate("2026-10-09", "short")`, `"Oct 9"`},
		{`@FormatDate("2026-10-09", "long")`, `"Friday, October 9, 2026"`},
		{`@FormatDate("2026-10-09", "weekday")`, `"Fri"`},
		{`@FormatDate("2026-10-09", "month")`, `"Oct 2026"`},
		{`@FormatDate("2026-10-10T04:30:00Z", "time")`, `"9:30 PM"`},
		{`@FormatDate("2026-10-10T04:30:00Z", "datetime")`, `"Oct 9, 9:30 PM"`},
		{`@FormatDate("2026-10-10T04:30:00Z", "iso")`, `"2026-10-09"`},
		{`@FormatDate(1791606600000, "iso")`, `"2026-10-09"`},
		{`@FormatDate("2026-02-30")`, `null`},
		{`@FormatDate("soon")`, `null`},
		{`@AddDays("2026-12-30", 3)`, `"2027-01-02"`},
		{`@AddDays("2026-03-01", -1)`, `"2026-02-28"`},
		{`@AddMonths("2026-01-31", 1)`, `"2026-02-28"`},
		{`@AddMonths("2026-01-31", 12)`, `"2027-01-31"`},
		{`@AddMonths("2026-03-15", -3)`, `"2025-12-15"`},
		{`@DaysBetween("2026-10-01", "2026-10-09")`, `8`},
		{`@DaysBetween("2026-10-09", "2026-10-01")`, `-8`},
		{`@MonthsBetween("2000-10-10", "2026-10-09")`, `311`},
		{`@Floor(@MonthsBetween("2000-10-09", @Today()) / 12)`, `26`},
		{`@MonthsBetween("2026-10-09", "2026-08-10")`, `-1`},
		{`@StartOf("2026-10-09", "week")`, `"2026-10-05"`},
		{`@StartOf("2026-10-09", "month")`, `"2026-10-01"`},
		{`@StartOf("2026-10-09", "year")`, `"2026-01-01"`},

		{`@GroupBy(` + rows + `, "team", "n")`, `[{"key":"a","value":5},{"key":"b","value":5}]`},
		{`@GroupBy(` + rows + `, "team")`, `[{"key":"a","value":2},{"key":"b","value":1}]`},
		{`@GroupBy(` + rows + `, "team", "n", "max")`, `[{"key":"a","value":3},{"key":"b","value":5}]`},
		{`@GroupBy([{k: null}, {k: ""}], "k")`, `[{"key":"Unknown","value":2}]`},
		{`@Pivot([{d: "Mon", who: "x", n: 1}, {d: "Mon", who: "y", n: 2}, {d: "Tue", who: "x", n: 4}], "d", "who", "n")`, `[{"key":"Mon","x":1,"y":2},{"key":"Tue","x":4,"y":0}]`},
		{`@Top(` + rows + `, "n", 2)`, `[{"n":5,"on":"2026-10-07","team":"b"},{"n":3,"on":"2026-10-19","team":"a"}]`},
		{`@Count(@Top(` + rows + `, "n", 1, "asc"))`, `1`},
		{`@Top([{n: null, id: 1}, {n: 1, id: 2}], "n", 2)`, `[{"id":2,"n":1},{"id":1,"n":null}]`},
		{`@Count(@Between(` + rows + `, "n", 3))`, `2`},
		{`@Count(@Between(` + rows + `, "on", "2026-10-06", "2026-10-19"))`, `2`},
		{`@Count(@Between(` + rows + `, "team", null, "a"))`, `2`},
		{`@Bucket(` + rows + `, "on", "week", "n")`, `[{"key":"2026-10-05","value":7},{"key":"2026-10-12","value":0},{"key":"2026-10-19","value":3}]`},
		{`@Bucket(` + rows + `, "on", "month")`, `[{"key":"2026-10","value":3}]`},
		{`@Bucket(` + rows + `, "on", "week", "n", "avg")`, `[{"key":"2026-10-05","value":3.5},{"key":"2026-10-19","value":3}]`},
		{`@Cumulative([1, 2, 3], "")`, `[1,3,6]`},
		{`@Cumulative([{n: 1}, {n: 2}], "n")`, `[{"n":1},{"n":3}]`},
		{`@MovingAvg([2, 4, 6, 8], "", 2)`, `[2,3,5,7]`},
		{`@MovingAvg([1], "", 0)`, `null`},
		{`@Change(120, 100)`, `20`},
		{`@Change(1, 0)`, `null`},
		{`@Histogram([1, 2, 2, 9, 10], 3)`, `[{"key":"1","value":3},{"key":"4","value":0},{"key":"7","value":2}]`},
		{`@Histogram([1], 0)`, `null`},
		{`@Stats([2, 4, 4, 4, 5, 5, 7, 9])`, `{"count":8,"max":9,"mean":5,"median":4.5,"min":2,"stdev":2}`},
		{`@Stats([]).count`, `0`},
	}
	lib := library(t)
	for _, tc := range cases {
		if got := value(t, lib, tc.expression); got != tc.want {
			t.Errorf("%s\n got %s\nwant %s", tc.expression, got, tc.want)
		}
	}
}

func TestEveryListedFunctionIsImplementedAndTested(t *testing.T) {
	lib := library(t)
	names := []string{"Append", "Prepend", "Concat", "Remove", "RemoveAt", "Update", "Toggle", "Includes", "Find", "IndexOf", "Take", "Slice", "Unique", "Reverse", "Range", "Pick", "Shuffle", "Flatten", "Chunk",
		"Pow", "Sqrt", "Random", "Clamp", "Percent", "Lerp", "Allocate", "Payment", "Convert",
		"Format", "Join", "Plural", "Pad", "Upper", "Lower", "Trim", "Capitalize", "Split", "Replace", "Duration", "Default", "MixColor",
		"Now", "Today", "FormatDate", "AddDays", "AddMonths", "DaysBetween", "MonthsBetween", "StartOf",
		"GroupBy", "Pivot", "Top", "Between", "Bucket", "Cumulative", "MovingAvg", "Change", "Histogram", "Stats"}
	if len(names) != 59 {
		t.Fatalf("%d names", len(names))
	}
	for _, name := range names {
		program := lib.Parse("$x = 0\nroot = Col([@Set($x, @" + name + "())])")
		session := openuilang.NewSession()
		session.Load(program)
		var reports []string
		root, problems := session.Render()
		for _, problem := range append(program.Errors, problems...) {
			reports = append(reports, problem.Message)
		}
		if root == nil || strings.Contains(strings.Join(reports, "\n"), "no implementation") || strings.Contains(strings.Join(reports, "\n"), "Unknown") {
			t.Errorf("@%s: %v", name, reports)
		}
	}
}

func TestRandomFunctionsGiveValuesOnlyInsideActions(t *testing.T) {
	lib := library(t)
	for _, expression := range []string{`@Random(1, 6, true)`, `@Pick(["a", "b"])`, `@Shuffle([1, 2, 3])`} {
		if got := value(t, lib, expression); got != "null" {
			t.Errorf("%s gave %s while rendering", expression, got)
		}
	}
	session := openuilang.NewSession()
	session.Load(lib.Parse("$roll = 0\n$pick = \"\"\n$deck = []\n$part = 0\nroot = Col([@Action([@Set($roll, @Random(1, 6, true)), @Set($pick, @Pick([\"a\", \"b\"])), @Set($deck, @Shuffle([1, 2, 3])), @Set($part, @Random(10, 20))])])"))
	root, _ := session.Render()
	children, _ := root.Props.Get("children")
	session.Trigger("", "", children.([]any)[0])
	for name, want := range map[string]string{"$roll": `4`, "$pick": `"b"`, "$deck": `[2,3,1]`, "$part": `15`} {
		got, _ := session.Get(name)
		if out, _ := json.Marshal(plain(got)); string(out) != want {
			t.Errorf("%s is %s, want %s", name, out, want)
		}
	}
}

func TestDatesWithoutAZoneAreReadInTheReadersZone(t *testing.T) {
	lib := library(t)
	for expression, want := range map[string]string{
		`@FormatDate("2026-10-09T23:30", "datetime")`:                                        `"Oct 9, 11:30 PM"`,
		`@FormatDate("2026-10-10T06:30:00Z", "datetime")`:                                    `"Oct 9, 11:30 PM"`,
		`@DaysBetween("2026-10-10T06:30:00Z", "2026-10-10")`:                                 `1`,
		`@AddDays("2026-10-10T06:30:00Z", 0)`:                                                `"2026-10-09"`,
		`@StartOf("2026-10-05T03:00:00Z", "week")`:                                           `"2026-09-28"`,
		`@Count(@Between([{at: "2026-10-10T06:30:00Z"}], "at", "2026-10-09", "2026-10-09"))`: `1`,
	} {
		if got := value(t, lib, expression); got != want {
			t.Errorf("%s\n got %s\nwant %s", expression, got, want)
		}
	}
}
