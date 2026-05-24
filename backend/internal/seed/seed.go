package seed

import (
	"database/sql"
	"log"
)

func Seed(db *sql.DB) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM releases").Scan(&count)
	if count > 0 {
		return
	}

	log.Println("Seeding demo data...")

	releases := []struct {
		title, artist, label string
		year                 int
	}{
		{"Kind of Blue", "Miles Davis", "Blue Note", 1959},
		{"A Love Supreme", "John Coltrane", "Impulse!", 1965},
		{"Exodus", "Bob Marley", "Island", 1977},
		{"Purple Rain", "Prince", "Warner", 1984},
		{"Rumours", "Fleetwood Mac", "Warner", 1977},
		{"Blue", "Joni Mitchell", "Reprise", 1971},
		{"In the Wee Small Hours", "Frank Sinatra", "Capitol", 1955},
		{"Sketches of Spain", "Miles Davis", "Columbia", 1960},
		{"Getz / Gilberto", "Stan Getz & João Gilberto", "Verve", 1964},
	}

	ids := make([]string, len(releases))
	for i, r := range releases {
		var id string
		err := db.QueryRow(
			`INSERT INTO releases (title, artist, label, year) VALUES (?, ?, ?, ?) RETURNING id`,
			r.title, r.artist, r.label, r.year,
		).Scan(&id)
		if err != nil {
			log.Printf("seed release %q: %v", r.title, err)
			continue
		}
		ids[i] = id
	}

	// Demo user
	var userID string
	db.QueryRow(`INSERT INTO users (email, display_name) VALUES ('demo@cratekeeper.com', 'Demo Digger') RETURNING id`).Scan(&userID)

	// Collection items
	items := []struct {
		releaseIdx      int
		media, pressing string
		price           float64
	}{
		{0, "VG+", "Original", 320},
		{1, "M", "Repress", 45},
		{2, "VG", "UK 1st", 80},
		{3, "VG+", "US 1st", 65},
		{4, "VG", "Original", 40},
		{5, "VG+", "CA 1st", 110},
	}
	for _, it := range items {
		if ids[it.releaseIdx] == "" {
			continue
		}
		db.Exec(
			`INSERT INTO collection_items (user_id, release_id, media_condition, notes, purchase_price) VALUES (?, ?, ?, ?, ?)`,
			userID, ids[it.releaseIdx], it.media, it.pressing, it.price,
		)
	}

	// Wishlist
	wishes := []struct {
		releaseIdx  int
		priority    int
		targetPrice float64
		notes       string
	}{
		{6, 2, 80, "OG Capitol pressing only"},
		{7, 5, 45, "Any pressing, good condition"},
		{8, 2, 120, "US Verve original"},
	}
	for _, w := range wishes {
		if ids[w.releaseIdx] == "" {
			continue
		}
		db.Exec(
			`INSERT INTO wishlist_items (user_id, release_id, priority, target_price, pressing_notes) VALUES (?, ?, ?, ?, ?)`,
			userID, ids[w.releaseIdx], w.priority, w.targetPrice, w.notes,
		)
	}

	log.Println("Demo data seeded.")
}
