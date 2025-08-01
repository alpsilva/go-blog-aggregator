-- +goose Up
CREATE TABLE feeds (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    user_id UUID NOT NULL,
    name TEXT  NOT NULL,
    url VARCHAR UNIQUE NOT NULL
);

ALTER TABLE feeds
ADD CONSTRAINT fk_user_id FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE feeds DROP CONSTRAINT fk_user_id;
DROP TABLE feeds;