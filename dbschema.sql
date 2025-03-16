CREATE TABLE users (
    user_id BIGINT PRIMARY KEY,
    is_admin BOOLEAN NOT NULL CHECK(is_admin IN (0, 1))
);

CREATE TABLE messages (
    message_id BIGINT PRIMARY KEY,
    -- date stored as unix time
    date UNSIGNED BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE files (
    id INTEGER PRIMARY KEY,
    file_id TEXT NOT NULL,
    file_unique_id TEXT UNIQUE NOT NULL,
    file_name TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    FOREIGN KEY(message_id) REFERENCES messages(message_id),
    FOREIGN KEY(user_id) REFERENCES users(user_id)
);

