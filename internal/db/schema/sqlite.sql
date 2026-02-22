CREATE TABLE IF NOT EXISTS scribble_content (
    id INTEGER PRIMARY KEY, 
    doc TEXT NOT NULL,
    type TEXT GENERATED ALWAYS AS (json_extract(doc, '$.type')) STORED,
    slug TEXT GENERATED ALWAYS AS (json_extract(doc, '$.properties.slug[0]')) STORED,
    deleted INTEGER GENERATED ALWAYS AS (CASE WHEN json_extract(doc, '$.properties.deleted[0]') = 1 THEN 1 ELSE 0 END) STORED,
    status TEXT GENERATED ALWAYS AS (json_extract(doc, '$.properties."post-status"[0]')) STORED,
    visibility TEXT GENERATED ALWAYS AS (json_extract(doc, '$.properties.visibility[0]')) STORED,
    is_unlisted INTEGER GENERATED ALWAYS AS (CASE WHEN visibility = 'unlisted' THEN 1 ELSE 0 END) STORED,
    is_visible INTEGER GENERATED ALWAYS AS (
        CASE WHEN 
                deleted = 0 
                AND (status is null OR status = 'published') 
                AND (visibility is null OR visibility = 'public' OR visibility = 'unlisted') 
            THEN 1 
            ELSE 0 
        END
    ) STORED,
    created_at TEXT GENERATED ALWAYS AS (json_extract(doc, '$.properties.created_at[0]')) STORED,
    created_year INTEGER GENERATED ALWAYS AS (CAST(strftime('%Y', created_at) AS INTEGER)) STORED,
    created_month INTEGER GENERATED ALWAYS AS (CAST(strftime('%m', created_at) AS INTEGER)) STORED,
    created_day INTEGER GENERATED ALWAYS AS (CAST(strftime('%d', created_at) AS INTEGER)) STORED,
    created_weekday INTEGER GENERATED ALWAYS AS (CAST(strftime('%w', created_at) AS INTEGER)) STORED,
    created_week INTEGER GENERATED ALWAYS AS (CAST(strftime('%W', created_at) AS INTEGER)) STORED,
    created_day_of_year INTEGER GENERATED ALWAYS AS (CAST(strftime('%j', created_at) AS INTEGER)) STORED,
    updated_at TEXT GENERATED ALWAYS AS (json_extract(doc, '$.properties.updated_at[0]')) STORED
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_slug ON scribble_content(slug);

CREATE INDEX IF NOT EXISTS idx_created_at ON scribble_content(created_at);

CREATE INDEX IF NOT EXISTS idx_updated_at ON scribble_content(updated_at);

CREATE TABLE IF NOT EXISTS scribble_categories (
    doc_id INTEGER NOT NULL, 
    category TEXT NOT NULL,
    PRIMARY KEY (doc_id, category),
    FOREIGN KEY (doc_id) REFERENCES scribble_content(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_category_value ON scribble_categories(category);

CREATE INDEX IF NOT EXISTS idx_category_cover ON scribble_categories(category, doc_id);

CREATE TRIGGER IF NOT EXISTS trg_add_categories
AFTER INSERT ON scribble_content
BEGIN
    INSERT INTO scribble_categories (doc_id, category) SELECT NEW.id, json_each.value FROM json_each(NEW.doc, '$.properties.category');
END;

CREATE TRIGGER IF NOT EXISTS trg_update_categories
AFTER UPDATE OF doc ON scribble_content
WHEN json_extract(OLD.doc, '$.properties.category') IS NOT json_extract(NEW.doc, '$.properties.category')
BEGIN
    DELETE FROM scribble_categories WHERE doc_id = OLD.id;
    INSERT INTO scribble_categories (doc_id, category) SELECT NEW.id, json_each.value FROM json_each(NEW.doc, '$.properties.category');
END;