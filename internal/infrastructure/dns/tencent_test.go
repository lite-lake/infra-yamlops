package dns

import (
	"testing"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

func TestTencentRecordMapper_MapsLineToInternal(t *testing.T) {
	mapper := &tencentRecordMapper{}

	cases := []struct {
		name string
		line string
		want string
	}{
		{name: "默认线路", line: "默认", want: "default"},
		{name: "境外线路", line: "境外", want: "oversea"},
		{name: "空线路归一为默认", line: "", want: ""},
		{name: "未知线路原样保留", line: "电信", want: "电信"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapper.mapRecord(&dnspod.RecordListItem{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("@"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.2.3.4"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr(tc.line),
			})
			if got.Line != tc.want {
				t.Fatalf("line = %q, want %q", got.Line, tc.want)
			}
		})
	}
}

func TestToTencentLine(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: "默认"},
		{in: "default", want: "默认"},
		{in: "oversea", want: "境外"},
		{in: "境外", want: "境外"},
	}
	for _, tc := range cases {
		if got := toTencentLine(tc.in); got != tc.want {
			t.Fatalf("toTencentLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
