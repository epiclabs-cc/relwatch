package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by the single-row lookups when no record matches.
var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

type Project struct {
	ID          int64
	Name        string
	Provider    string
	Repo        string
	InstanceURL string
	CreatedAt   time.Time
}

type Release struct {
	ID           int64
	ProjectID    int64
	Tag          string
	Name         string
	PublishedAt  *time.Time
	URL          string
	IsLatest     bool
	IsPrerelease bool
	CreatedAt    time.Time
}

// ContainerImage is a tracked OCI Distribution v2 image repository.
type ContainerImage struct {
	ID        int64
	Name      string
	Registry  string
	Repo      string
	CreatedAt time.Time
}

// ImageDigest is the last observed digest for one tag of a tracked image.
// A moved tag (digest change) is treated as a brand-new release (v0.4.0 Q3).
type ImageDigest struct {
	ID         int64
	ImageID    int64
	Tag        string
	Digest     string
	IsLatest   bool
	ObservedAt *time.Time
	CreatedAt  time.Time
}

// ImageSummary rolls up the latest known digest per tracked image.
type ImageSummary struct {
	ContainerImage
	LatestTag        string
	LatestDigest     string
	LatestObservedAt *time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CountReleases(projectID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM releases WHERE project_id = ?`, projectID).Scan(&n)
	return n, err
}

// DeleteProject removes a project and, via the releases.project_id
// ON DELETE CASCADE + foreign_keys(ON) pragma, its entire release history.
func (s *Store) DeleteProject(id int64) error {
	res, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no tracked project with id %d: %w", id, ErrNotFound)
	}
	return nil
}

func (s *Store) AddContainerImage(name, registry, repo string) (ContainerImage, error) {
	res, err := s.db.Exec(`INSERT INTO container_images (name, registry, repo) VALUES (?,?,?)`, name, registry, repo)
	if err != nil {
		return ContainerImage{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ContainerImage{}, err
	}
	return ContainerImage{ID: id, Name: name, Registry: registry, Repo: repo}, nil
}

func (s *Store) ListContainerImages() ([]ContainerImage, error) {
	rows, err := s.db.Query(`SELECT id, name, registry, repo, created_at FROM container_images ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []ContainerImage
	for rows.Next() {
		var i ContainerImage
		var createdAt int64
		if err := rows.Scan(&i.ID, &i.Name, &i.Registry, &i.Repo, &createdAt); err != nil {
			return nil, err
		}
		i.CreatedAt = time.Unix(createdAt, 0)
		images = append(images, i)
	}
	return images, rows.Err()
}

func (s *Store) DeleteContainerImage(id int64) error {
	res, err := s.db.Exec(`DELETE FROM container_images WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no tracked image with id %d: %w", id, ErrNotFound)
	}
	return nil
}

func (s *Store) ListImageSummaries() ([]ImageSummary, error) {
	return s.imageSummaries(0)
}

// GetImageSummary returns one image's summary, or ErrNotFound.
func (s *Store) GetImageSummary(id int64) (ImageSummary, error) {
	summaries, err := s.imageSummaries(id)
	if err != nil {
		return ImageSummary{}, err
	}
	if len(summaries) == 0 {
		return ImageSummary{}, ErrNotFound
	}
	return summaries[0], nil
}

// imageSummaries rolls up the latest observed digest per image; id > 0 selects one.
func (s *Store) imageSummaries(id int64) ([]ImageSummary, error) {
	q := `
		SELECT ci.id, ci.name, ci.registry, ci.repo, ci.created_at,
		       d.tag, d.digest, d.observed_at
		FROM container_images ci
		LEFT JOIN image_digests d ON d.id = (
			SELECT d2.id FROM image_digests d2
			WHERE d2.image_id = ci.id
			ORDER BY d2.observed_at DESC
			LIMIT 1
		)`
	var args []any
	if id > 0 {
		q += ` WHERE ci.id = ?`
		args = append(args, id)
	}
	q += ` ORDER BY ci.name`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []ImageSummary
	for rows.Next() {
		var is ImageSummary
		var createdAt int64
		var tag, digest sql.NullString
		var observedAt sql.NullInt64
		if err := rows.Scan(&is.ID, &is.Name, &is.Registry, &is.Repo, &createdAt, &tag, &digest, &observedAt); err != nil {
			return nil, err
		}
		is.CreatedAt = time.Unix(createdAt, 0)
		is.LatestTag = tag.String
		is.LatestDigest = digest.String
		if observedAt.Valid {
			t := time.Unix(observedAt.Int64, 0)
			is.LatestObservedAt = &t
		}
		summaries = append(summaries, is)
	}
	return summaries, rows.Err()
}

func (s *Store) CountImageDigests(imageID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM image_digests WHERE image_id = ?`, imageID).Scan(&n)
	return n, err
}

func (s *Store) UpsertImageDigest(d ImageDigest) (moved bool, err error) {
	// A brand-new tag counts as a moved/new release (v0.4.0).
	res, err := s.db.Exec(`INSERT INTO image_digests (image_id, tag, digest, is_latest, observed_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT (image_id, tag) DO NOTHING`,
		d.ImageID, d.Tag, d.Digest, boolInt(d.IsLatest), unix(d.ObservedAt))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 1 {
		return true, nil
	}
	// Tag already tracked: only a moved digest (new digest on that tag) is a release.
	res, err = s.db.Exec(`UPDATE image_digests
		SET digest = ?, is_latest = ?, observed_at = ?
		WHERE image_id = ? AND tag = ? AND digest != ?`,
		d.Digest, boolInt(d.IsLatest), unix(d.ObservedAt), d.ImageID, d.Tag, d.Digest)
	if err != nil {
		return false, err
	}
	n, err = res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) AddProject(name, provider, repo, instanceURL string) (Project, error) {
	res, err := s.db.Exec(`INSERT INTO projects (name, provider, repo, instance_url) VALUES (?,?,?,?)`,
		name, provider, repo, instanceURL)
	if err != nil {
		return Project{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	return Project{ID: id, Name: name, Provider: provider, Repo: repo, InstanceURL: instanceURL}, nil
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, name, provider, repo, instance_url, created_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Provider, &p.Repo, &p.InstanceURL, &createdAt); err != nil {
			return nil, err
		}
		p.CreatedAt = time.Unix(createdAt, 0)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

type ProjectSummary struct {
	Project
	LatestTag        string
	LatestReleasedAt *time.Time
	LatestURL        string
}

func (s *Store) ListProjectSummaries() ([]ProjectSummary, error) {
	return s.projectSummaries(0)
}

// GetProjectSummary returns one project's summary, or ErrNotFound.
func (s *Store) GetProjectSummary(id int64) (ProjectSummary, error) {
	summaries, err := s.projectSummaries(id)
	if err != nil {
		return ProjectSummary{}, err
	}
	if len(summaries) == 0 {
		return ProjectSummary{}, ErrNotFound
	}
	return summaries[0], nil
}

// projectSummaries rolls up the latest release per project; id > 0 selects one.
func (s *Store) projectSummaries(id int64) ([]ProjectSummary, error) {
	q := `
		SELECT p.id, p.name, p.provider, p.repo, p.instance_url, p.created_at,
		       r.tag, r.published_at, r.url
		FROM projects p
		LEFT JOIN releases r ON r.id = (
			SELECT r2.id FROM releases r2
			WHERE r2.project_id = p.id
			ORDER BY COALESCE(r2.published_at, 0) DESC, r2.id DESC
			LIMIT 1
		)`
	var args []any
	if id > 0 {
		q += ` WHERE p.id = ?`
		args = append(args, id)
	}
	q += ` ORDER BY p.name`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []ProjectSummary
	for rows.Next() {
		var s ProjectSummary
		var createdAt int64
		var publishedAt sql.NullInt64
		var tag, url sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.Provider, &s.Repo, &s.InstanceURL, &createdAt, &tag, &publishedAt, &url); err != nil {
			return nil, err
		}
		s.CreatedAt = time.Unix(createdAt, 0)
		s.LatestTag = tag.String
		s.LatestURL = url.String
		if publishedAt.Valid {
			t := time.Unix(publishedAt.Int64, 0)
			s.LatestReleasedAt = &t
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// ListReleases returns a project's full release history, newest first.
func (s *Store) ListReleases(projectID int64) ([]Release, error) {
	rows, err := s.db.Query(`
		SELECT id, project_id, tag, name, published_at, url, is_latest, is_prerelease, created_at
		FROM releases
		WHERE project_id = ?
		ORDER BY COALESCE(published_at, 0) DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		var publishedAt sql.NullInt64
		var createdAt int64
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Tag, &r.Name, &publishedAt, &r.URL, &r.IsLatest, &r.IsPrerelease, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(createdAt, 0)
		if publishedAt.Valid {
			t := time.Unix(publishedAt.Int64, 0)
			r.PublishedAt = &t
		}
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

// ListImageDigests returns an image's digest history, newest first.
func (s *Store) ListImageDigests(imageID int64) ([]ImageDigest, error) {
	rows, err := s.db.Query(`
		SELECT id, image_id, tag, digest, is_latest, observed_at, created_at
		FROM image_digests
		WHERE image_id = ?
		ORDER BY COALESCE(observed_at, 0) DESC, id DESC`, imageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var digests []ImageDigest
	for rows.Next() {
		var d ImageDigest
		var observedAt sql.NullInt64
		var createdAt int64
		if err := rows.Scan(&d.ID, &d.ImageID, &d.Tag, &d.Digest, &d.IsLatest, &observedAt, &createdAt); err != nil {
			return nil, err
		}
		d.CreatedAt = time.Unix(createdAt, 0)
		if observedAt.Valid {
			t := time.Unix(observedAt.Int64, 0)
			d.ObservedAt = &t
		}
		digests = append(digests, d)
	}
	return digests, rows.Err()
}

func (s *Store) UpsertRelease(r Release) (inserted bool, err error) {
	res, err := s.db.Exec(`INSERT INTO releases (project_id, tag, name, published_at, url, is_latest, is_prerelease)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT (project_id, tag) DO NOTHING`,
		r.ProjectID, r.Tag, r.Name, unix(r.PublishedAt), r.URL, boolInt(r.IsLatest), boolInt(r.IsPrerelease))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 1 {
		return true, nil
	}

	_, err = s.db.Exec(`UPDATE releases
		SET name = ?, published_at = ?, url = ?, is_latest = ?
		WHERE project_id = ? AND tag = ?`,
		r.Name, unix(r.PublishedAt), r.URL, boolInt(r.IsLatest), r.ProjectID, r.Tag)
	return false, err
}

func unix(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	v := t.Unix()
	return &v
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

type Feed struct {
	ID        int64
	Name      string
	URL       string
	CreatedAt time.Time
}

type FeedRelease struct {
	ID          int64
	FeedID      int64
	Tag         string
	Name        string
	PublishedAt *time.Time
	URL         string
	IsLatest    bool
	CreatedAt   time.Time
}

type FeedSummary struct {
	Feed
	LatestTag        string
	LatestReleasedAt *time.Time
	LatestURL        string
}

func (s *Store) AddFeed(name, url string) (Feed, error) {
	res, err := s.db.Exec(`INSERT INTO feeds (name, url) VALUES (?,?)`, name, url)
	if err != nil {
		return Feed{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Feed{}, err
	}
	return Feed{ID: id, Name: name, URL: url}, nil
}

func (s *Store) ListFeeds() ([]Feed, error) {
	rows, err := s.db.Query(`SELECT id, name, url, created_at FROM feeds ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		var f Feed
		var createdAt int64
		if err := rows.Scan(&f.ID, &f.Name, &f.URL, &createdAt); err != nil {
			return nil, err
		}
		f.CreatedAt = time.Unix(createdAt, 0)
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func (s *Store) DeleteFeed(id int64) error {
	res, err := s.db.Exec(`DELETE FROM feeds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no tracked feed with id %d: %w", id, ErrNotFound)
	}
	return nil
}

func (s *Store) CountFeedReleases(feedID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM feed_releases WHERE feed_id = ?`, feedID).Scan(&n)
	return n, err
}

func (s *Store) ListFeedReleases(feedID int64) ([]FeedRelease, error) {
	rows, err := s.db.Query(`SELECT id, feed_id, tag, name, published_at, url, is_latest, created_at
		FROM feed_releases
		WHERE feed_id = ?
		ORDER BY COALESCE(published_at, 0) DESC, id DESC`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []FeedRelease
	for rows.Next() {
		var r FeedRelease
		var publishedAt sql.NullInt64
		var createdAt int64
		if err := rows.Scan(&r.ID, &r.FeedID, &r.Tag, &r.Name, &publishedAt, &r.URL, &r.IsLatest, &createdAt); err != nil {
			return nil, err
		}
		if publishedAt.Valid {
			t := time.Unix(publishedAt.Int64, 0)
			r.PublishedAt = &t
		}
		r.CreatedAt = time.Unix(createdAt, 0)
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func (s *Store) ListFeedSummaries() ([]FeedSummary, error) {
	return s.feedSummaries(0)
}

// GetFeedSummary returns one feed's summary, or ErrNotFound.
func (s *Store) GetFeedSummary(id int64) (FeedSummary, error) {
	summaries, err := s.feedSummaries(id)
	if err != nil {
		return FeedSummary{}, err
	}
	if len(summaries) == 0 {
		return FeedSummary{}, ErrNotFound
	}
	return summaries[0], nil
}

// feedSummaries rolls up the latest release per feed; id > 0 selects one.
func (s *Store) feedSummaries(id int64) ([]FeedSummary, error) {
	q := `
		SELECT f.id, f.name, f.url, f.created_at,
		       fr.tag, fr.published_at, fr.url
		FROM feeds f
		LEFT JOIN feed_releases fr ON fr.id = (
			SELECT fr2.id FROM feed_releases fr2
			WHERE fr2.feed_id = f.id
			ORDER BY COALESCE(fr2.published_at, 0) DESC, fr2.id DESC
			LIMIT 1
		)`
	var args []any
	if id > 0 {
		q += ` WHERE f.id = ?`
		args = append(args, id)
	}
	q += ` ORDER BY f.name`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []FeedSummary
	for rows.Next() {
		var s FeedSummary
		var createdAt int64
		var publishedAt sql.NullInt64
		var tag, url sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &createdAt, &tag, &publishedAt, &url); err != nil {
			return nil, err
		}
		s.CreatedAt = time.Unix(createdAt, 0)
		s.LatestTag = tag.String
		s.LatestURL = url.String
		if publishedAt.Valid {
			t := time.Unix(publishedAt.Int64, 0)
			s.LatestReleasedAt = &t
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (s *Store) UpsertFeedRelease(r FeedRelease) (inserted bool, err error) {
	res, err := s.db.Exec(`INSERT INTO feed_releases (feed_id, tag, name, published_at, url, is_latest)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT (feed_id, tag) DO NOTHING`,
		r.FeedID, r.Tag, r.Name, unix(r.PublishedAt), r.URL, boolInt(r.IsLatest))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 1 {
		return true, nil
	}

	_, err = s.db.Exec(`UPDATE feed_releases
		SET name = ?, published_at = ?, url = ?, is_latest = ?
		WHERE feed_id = ? AND tag = ?`,
		r.Name, unix(r.PublishedAt), r.URL, boolInt(r.IsLatest), r.FeedID, r.Tag)
	return false, err
}
