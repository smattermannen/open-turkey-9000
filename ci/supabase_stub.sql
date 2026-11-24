-- Minimal stand-ins for Supabase's auth and storage, to check schema.sql.
create role anon nologin; create role authenticated nologin; create role service_role nologin bypassrls;
create schema auth; create schema storage;
create table auth.users (id uuid primary key, email text);
create function auth.uid() returns uuid language sql stable as $$ select nullif(current_setting('request.jwt.claim.sub', true), '')::uuid $$;
create function auth.email() returns text language sql stable as $$ select nullif(current_setting('request.jwt.claim.email', true), '') $$;
create table storage.buckets (id text primary key, name text, public boolean, file_size_limit bigint, allowed_mime_types text[]);
create table storage.objects (bucket_id text, name text);
alter table storage.objects enable row level security;
grant usage on schema public, auth to anon, authenticated, service_role;
grant execute on function auth.uid(), auth.email() to anon, authenticated, service_role;
-- Supabase grants table rights to these roles; row-level security decides.
alter default privileges in schema public grant all on tables to anon, authenticated, service_role;
alter default privileges in schema public grant all on sequences to anon, authenticated, service_role;
insert into auth.users values ('00000000-0000-0000-0000-000000000001', 'me@example.org'),
  ('00000000-0000-0000-0000-000000000002', 'anna@example.org'), ('00000000-0000-0000-0000-000000000003', 'bob@example.org'),
  ('00000000-0000-0000-0000-000000000004', 'eve@example.org');
