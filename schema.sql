BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE "users" (
    "id"            serial PRIMARY KEY,
    "name"          varchar(40) NOT NULL,
    "email"         varchar(254),
    "password"      char(60),
    "created_at"    timestamp with time zone DEFAULT NOW() NOT NULL
);

CREATE TABLE "thread" (
    "id" uuid PRIMARY KEY
);

CREATE TABLE "message" (
    "id"        uuid    PRIMARY KEY,
    "thread_id" uuid    REFERENCES thread(id),
    "from"      integer REFERENCES users(id),
    "body"      TEXT
);

CREATE TABLE "groups" (
    "id" serial PRIMARY KEY
);

CREATE TABLE "group_user" (
    "group_id"  integer REFERENCES groups(id),
    "user_id"   integer REFERENCES users(id)
);

CREATE TABLE "user_thread" (
    "user_id"   integer REFERENCES users(id),
    "thread_id" uuid    REFERENCES thread(id)
);

CREATE TABLE "group_thread" (
    "group_id"  integer REFERENCES groups(id),
    "thread_id" uuid    REFERENCES thread(id)
);

COMMIT;
