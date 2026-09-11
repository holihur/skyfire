// Package update implements self-update for the skyfired binary: it resolves
// the latest (or a pinned) GitHub release, downloads the matching platform
// asset, verifies its sha256 against the release checksums, and atomically
// replaces the running executable.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultRepo is the GitHub repository that publishes skyfire releases.
const DefaultRepo = "holihur/skyfire"

// DefaultAPI is the GitHub REST API base URL.
const DefaultAPI = "https://api.github.com"

// maxDownload caps a single downloaded body (binary + checksums).
const maxDownload = 256 << 20

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of a GitHub release we care about.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

func (r *Release) find(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// Client resolves and downloads releases.
type Client struct {
	Repo   string
	API    string
	Token  string // optional GITHUB_TOKEN for higher rate limits
	HTTP   *http.Client
	GOOS   string
	GOARCH string
}

// New returns a Client for the given repository (empty falls back to
// DefaultRepo) targeting the current platform.
func New(repo string) *Client {
	if repo == "" {
		repo = DefaultRepo
	}
	return &Client{
		Repo:   repo,
		API:    DefaultAPI,
		HTTP:   &http.Client{Timeout: 5 * time.Minute},
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
	}
}

// AssetName returns the release asset name for the client's platform, e.g.
// "skyfired-linux-amd64" or "skyfired-windows-amd64.exe".
func (c *Client) AssetName() (string, error) {
	return assetName(c.GOOS, c.GOARCH)
}

func assetName(goos, goarch string) (string, error) {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return "", fmt.Errorf("unsupported OS %q", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture %q", goarch)
	}
	if goos == "windows" && goarch == "arm64" {
		return "", errors.New("no release artifact for windows/arm64")
	}
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("skyfired-%s-%s%s", goos, goarch, ext), nil
}

// Resolve returns the release for tag, or the latest release when tag is empty
// or "latest".
func (c *Client) Resolve(ctx context.Context, tag string) (*Release, error) {
	if tag == "" || tag == "latest" {
		return c.Latest(ctx)
	}
	return c.ByTag(ctx, tag)
}

// Latest fetches the latest published release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	return c.release(ctx, c.apiBase()+"/repos/"+c.Repo+"/releases/latest")
}

// ByTag fetches a specific release by tag.
func (c *Client) ByTag(ctx context.Context, tag string) (*Release, error) {
	return c.release(ctx, c.apiBase()+"/repos/"+c.Repo+"/releases/tags/"+tag)
}

func (c *Client) release(ctx context.Context, url string) (*Release, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}
	if rel.Tag == "" {
		return nil, errors.New("release has no tag_name")
	}
	return &rel, nil
}

// Fetch downloads the platform asset for rel and verifies it against the
// release's checksums.txt (when present).
func (c *Client) Fetch(ctx context.Context, rel *Release) ([]byte, error) {
	name, err := c.AssetName()
	if err != nil {
		return nil, err
	}
	asset, ok := rel.find(name)
	if !ok {
		return nil, fmt.Errorf("release %s has no asset %q", rel.Tag, name)
	}

	var want string
	if sums, ok := rel.find("checksums.txt"); ok {
		body, err := c.get(ctx, sums.URL)
		if err != nil {
			return nil, fmt.Errorf("download checksums: %w", err)
		}
		want = parseChecksums(string(body))[name]
		if want == "" {
			return nil, fmt.Errorf("checksums.txt has no entry for %q", name)
		}
	}

	data, err := c.get(ctx, asset.URL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	if want != "" {
		got := sha256Hex(data)
		if !strings.EqualFold(got, want) {
			return nil, fmt.Errorf("checksum mismatch for %s: expected %s, got %s", name, want, got)
		}
	}
	return data, nil
}

func (c *Client) apiBase() string {
	if c.API != "" {
		return strings.TrimRight(c.API, "/")
	}
	return DefaultAPI
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "skyfire-updater")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownload))
}

// Install atomically replaces target with data, creating the file with mode
// 0755. On Windows the running executable cannot be overwritten, so the old
// binary is moved aside first (a stale .old file may remain until reboot).
func Install(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".skyfired-new-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	discard := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		discard()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		discard()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		discard()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		discard()
		return fmt.Errorf("close temp file: %w", err)
	}

	if runtime.GOOS == "windows" {
		old := target + ".old"
		_ = os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			discard()
			return fmt.Errorf("move current binary: %w", err)
		}
		if err := os.Rename(tmpName, target); err != nil {
			_ = os.Rename(old, target)
			discard()
			return fmt.Errorf("replace binary: %w", err)
		}
		_ = os.Remove(old)
		return nil
	}

	if err := os.Rename(tmpName, target); err != nil {
		discard()
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// Newer reports whether latest is newer than current, and whether the two
// versions were comparable (they are not when either is not a release
// version, e.g. "dev").
func Newer(current, latest string) (newer, comparable bool) {
	cv, err := parseVersion(current)
	if err != nil {
		return false, false
	}
	lv, err := parseVersion(latest)
	if err != nil {
		return false, false
	}
	return compareSemver(cv, lv) < 0, true
}

func parseChecksums(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			out[fields[1]] = fields[0]
		}
	}
	return out
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type semver struct {
	nums []int
	pre  []string
}

func parseVersion(s string) (semver, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if s == "" {
		return semver{}, errors.New("empty version")
	}
	var v semver
	core := s
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		core = s[:i]
		rest := s[i:]
		if strings.HasPrefix(rest, "-") {
			rest = strings.TrimPrefix(rest, "-")
			if j := strings.IndexByte(rest, '+'); j >= 0 {
				rest = rest[:j]
			}
			if rest != "" {
				v.pre = strings.Split(rest, ".")
			}
		}
	}
	if core == "" {
		return semver{}, fmt.Errorf("invalid version %q", s)
	}
	for _, p := range strings.Split(core, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}, fmt.Errorf("invalid version %q", s)
		}
		v.nums = append(v.nums, n)
	}
	return v, nil
}

func compareSemver(a, b semver) int {
	if c := compareNums(a.nums, b.nums); c != 0 {
		return c
	}
	if len(a.pre) == 0 && len(b.pre) == 0 {
		return 0
	}
	if len(a.pre) == 0 {
		return 1 // release > pre-release
	}
	if len(b.pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.pre) || i < len(b.pre); i++ {
		if i >= len(a.pre) {
			return -1
		}
		if i >= len(b.pre) {
			return 1
		}
		x, y := a.pre[i], b.pre[i]
		xn, xe := strconv.Atoi(x)
		yn, ye := strconv.Atoi(y)
		switch {
		case xe == nil && ye == nil:
			if xn != yn {
				return sign(xn - yn)
			}
		case xe == nil:
			return -1 // numeric identifiers sort before alphanumeric
		case ye == nil:
			return 1
		default:
			if x != y {
				if x < y {
					return -1
				}
				return 1
			}
		}
	}
	return 0
}

func compareNums(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return sign(x - y)
		}
	}
	return 0
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
