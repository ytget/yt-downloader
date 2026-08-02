package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	if compareSemver("1.2.0", "1.1.9") <= 0 {
		t.Fatal("1.2.0 > 1.1.9")
	}
	if compareSemver("1.0.0", "1.0.0") != 0 {
		t.Fatal("eq")
	}
	if compareSemver("0.9.0", "1.0.0") >= 0 {
		t.Fatal("less")
	}
}

func TestCheckLatestURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://example.com/r","draft":false,"prerelease":false}`))
	}))
	defer srv.Close()

	res := CheckLatestURL(context.Background(), "1.0.0", srv.URL)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if !res.UpdateAvailable || res.Latest != "1.2.3" {
		t.Fatalf("%+v", res)
	}
}

func TestSkipDev(t *testing.T) {
	res := CheckLatest(context.Background(), "dev")
	if res.Checked {
		t.Fatal("dev should not check")
	}
}
