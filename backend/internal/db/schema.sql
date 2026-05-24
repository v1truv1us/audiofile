PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    email       TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    password_hash TEXT,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS passkeys (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   BLOB NOT NULL UNIQUE,
    public_key      BLOB NOT NULL,
    sign_count      INTEGER NOT NULL DEFAULT 0,
    name            TEXT NOT NULL DEFAULT 'Passkey',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    last_used_at    DATETIME
);

-- Global release metadata (ADR-003: separate from ownership)
CREATE TABLE IF NOT EXISTS releases (
    id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    discogs_id  TEXT UNIQUE,
    title       TEXT NOT NULL,
    artist      TEXT NOT NULL,
    year        INTEGER,
    label       TEXT,
    catalog_no  TEXT,
    format      TEXT,   -- LP, EP, 45, etc.
    country     TEXT,
    genres      TEXT,   -- JSON array
    cover_url   TEXT,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- User-owned collection items (ADR-002: user-scoped)
CREATE TABLE IF NOT EXISTS collection_items (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    release_id      TEXT NOT NULL REFERENCES releases(id),
    media_condition TEXT NOT NULL DEFAULT 'VG',  -- M/NM/VG+/VG/G+/G/F/P
    sleeve_condition TEXT NOT NULL DEFAULT 'VG',
    purchase_price  REAL,
    purchase_date   DATE,
    notes           TEXT,
    is_for_sale     INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Condition history log
CREATE TABLE IF NOT EXISTS condition_history (
    id                  TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    collection_item_id  TEXT NOT NULL REFERENCES collection_items(id) ON DELETE CASCADE,
    media_condition     TEXT NOT NULL,
    sleeve_condition    TEXT NOT NULL,
    warp_notes          TEXT,
    scratch_notes       TEXT,
    cleaning_notes      TEXT,
    playback_notes      TEXT,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Wishlist
CREATE TABLE IF NOT EXISTS wishlist_items (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    release_id      TEXT REFERENCES releases(id),
    -- Manual entry if no Discogs match
    manual_title    TEXT,
    manual_artist   TEXT,
    priority        INTEGER NOT NULL DEFAULT 5,  -- 1 (highest) to 10 (lowest)
    target_price    REAL,
    pressing_notes  TEXT,
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Photos attached to collection items
CREATE TABLE IF NOT EXISTS item_photos (
    id                  TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    collection_item_id  TEXT NOT NULL REFERENCES collection_items(id) ON DELETE CASCADE,
    r2_key              TEXT NOT NULL,
    r2_thumbnail_key    TEXT,
    caption             TEXT,
    sort_order          INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- FTS5 index for search
CREATE VIRTUAL TABLE IF NOT EXISTS releases_fts USING fts5(
    title, artist, label, catalog_no,
    content=releases, content_rowid=rowid
);

CREATE TRIGGER IF NOT EXISTS releases_ai AFTER INSERT ON releases BEGIN
    INSERT INTO releases_fts(rowid, title, artist, label, catalog_no)
    VALUES (new.rowid, new.title, new.artist, new.label, new.catalog_no);
END;

CREATE TRIGGER IF NOT EXISTS releases_au AFTER UPDATE ON releases BEGIN
    INSERT INTO releases_fts(releases_fts, rowid, title, artist, label, catalog_no)
    VALUES ('delete', old.rowid, old.title, old.artist, old.label, old.catalog_no);
    INSERT INTO releases_fts(rowid, title, artist, label, catalog_no)
    VALUES (new.rowid, new.title, new.artist, new.label, new.catalog_no);
END;

CREATE TRIGGER IF NOT EXISTS releases_ad AFTER DELETE ON releases BEGIN
    INSERT INTO releases_fts(releases_fts, rowid, title, artist, label, catalog_no)
    VALUES ('delete', old.rowid, old.title, old.artist, old.label, old.catalog_no);
END;
