CREATE TABLE IF NOT EXISTS scribble_content (
    id INTEGER PRIMARY KEY, 
    doc TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_slug ON scribble_content(json_extract(doc, '$.properties.slug[0]'));

CREATE INDEX IF NOT EXISTS idx_doc_created ON scribble_content(json_extract(doc, '$.properties.created_at[0]'));

CREATE INDEX IF NOT EXISTS idx_doc_updated ON scribble_content(json_extract(doc, '$.properties.updated_at[0]'));

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