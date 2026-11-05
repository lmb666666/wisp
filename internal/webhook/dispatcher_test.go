package webhook

import "testing"

func TestStopIsIdempotentAndEmitAfterStopIsSafe(t *testing.T) {
	d := NewDispatcher(nil)
	d.Stop()
	d.Stop()
	// q 为 nil：如果 Emit 没在 stop 检查处返回，会在查询 webhook 时 panic。
	d.Emit("post.created", nil)
}
