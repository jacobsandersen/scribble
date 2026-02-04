-- name: GetDocumentBySlug :one
SELECT doc FROM scribble_content 
    WHERE slug = ?
    LIMIT 1;

-- name: ListDocuments :many
SELECT doc FROM scribble_content 
    ORDER BY created_at DESC 
    LIMIT ? OFFSET ?;

-- name: QueryDocuments :many
SELECT c.doc FROM scribble_content c
    WHERE (
        CAST(? AS INTEGER) = 0
        OR c.slug = ?
    )
        AND (
            CAST(? AS INTEGER) = 0
            OR EXISTS (
                SELECT 1
                FROM scribble_categories cat
                WHERE cat.doc_id = c.id
                    AND cat.category = ?
            )
        )
        AND (
            CAST(? AS INTEGER) = 0
            OR c.created_year = ?
        )
        AND (
            CAST(? AS INTEGER) = 0
            OR c.created_month = ?
        )
        AND (
            CAST(? AS INTEGER) = 0
            OR c.created_day = ?
        )
        AND (
            CAST(? AS INTEGER) = 0
            OR c.created_weekday = ?
        )
    ORDER BY c.created_at DESC
    LIMIT ? OFFSET ?;

-- name: ListCategories :many
SELECT DISTINCT category FROM scribble_categories 
    ORDER BY category ASC 
    LIMIT ? OFFSET ?;

-- name: ListCategoriesLike :many
SELECT DISTINCT category FROM scribble_categories 
    WHERE category LIKE ?
    ORDER BY category ASC 
    LIMIT ? OFFSET ?;

-- name: DocExistsBySlug :one
SELECT EXISTS (
    SELECT 1 FROM scribble_content 
        WHERE slug = ?
        LIMIT 1
);

-- name: UpdateDocumentBySlug :exec
UPDATE scribble_content 
    SET doc = ? 
    WHERE slug = ?;

-- name: InsertDocument :exec
INSERT INTO scribble_content (doc) VALUES (?);

-- name: DeleteDocumentBySlug :exec
DELETE FROM scribble_content WHERE slug = ?;

