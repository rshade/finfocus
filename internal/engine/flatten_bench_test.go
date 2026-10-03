package engine

import (
	"strconv"
	"testing"
)

// bigNested builds one property holding n nested objects, two to three levels
// deep. It stands in for a very large nested input: ConvertToProto walks every
// leaf before the 50-tag cap applies, so cost grows with the input.
func bigNested(n int) map[string]any {
	inner := make(map[string]any, n)
	for i := range n {
		inner["k"+strconv.Itoa(i)] = map[string]any{"a": "x", "b": map[string]any{"c": "y"}}
	}
	return map[string]any{"cfg": inner, "instanceType": "t3.micro"}
}

func BenchmarkConvertToProtoLarge(b *testing.B) {
	for _, n := range []int{50, 2000} {
		props := bigNested(n)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = ConvertToProto(props)
			}
		})
	}
}
