package util

import "testing"

func BenchmarkGetUUID(b *testing.B) {
	for i:=0; i< b.N; i++ {
		_ = GetUUID()
	}
}

func BenchmarkGetUUID_op(b *testing.B) {
	for i:=0; i< b.N; i++ {
		_ = GetUUID_op()
	}
}
