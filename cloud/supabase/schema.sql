-- KC9K's cloud on Supabase (SPEC §4.14, §6 2026-11-23; ROADMAP 2.10).
-- Run once in the project's SQL editor. Each KC9K account has one setup:
-- the private Storage object setups/<user id>/setup.json.gz and one row
-- in public.setups saying when and from which device it was uploaded.
-- Row-level security: an account reads and writes only its own. The anon
-- key the programs carry is public; never ship the service role key.

create table if not exists public.setups (
  user_id     uuid primary key references auth.users (id) on delete cascade,
  uploaded_at timestamptz not null,
  device      text not null default '' check (length(device) <= 200),
  format      integer not null,
  size        bigint not null check (size >= 0)
);

alter table public.setups enable row level security;

drop policy if exists "own setup row: read" on public.setups;
create policy "own setup row: read" on public.setups
  for select to authenticated using ((select auth.uid()) = user_id);
drop policy if exists "own setup row: add" on public.setups;
create policy "own setup row: add" on public.setups
  for insert to authenticated with check ((select auth.uid()) = user_id);
drop policy if exists "own setup row: change" on public.setups;
create policy "own setup row: change" on public.setups
  for update to authenticated using ((select auth.uid()) = user_id)
  with check ((select auth.uid()) = user_id);
drop policy if exists "own setup row: remove" on public.setups;
create policy "own setup row: remove" on public.setups
  for delete to authenticated using ((select auth.uid()) = user_id);

-- Private bucket, gzip only, at most 50 MB a setup.
insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values ('setups', 'setups', false, 52428800, array['application/gzip'])
on conflict (id) do update set public = false, file_size_limit = excluded.file_size_limit,
  allowed_mime_types = excluded.allowed_mime_types;

-- Objects: only <own user id>/setup.json.gz.
drop policy if exists "own setup object: read" on storage.objects;
create policy "own setup object: read" on storage.objects
  for select to authenticated
  using (bucket_id = 'setups' and name = (select auth.uid())::text || '/setup.json.gz');
drop policy if exists "own setup object: add" on storage.objects;
create policy "own setup object: add" on storage.objects
  for insert to authenticated
  with check (bucket_id = 'setups' and name = (select auth.uid())::text || '/setup.json.gz');
drop policy if exists "own setup object: change" on storage.objects;
create policy "own setup object: change" on storage.objects
  for update to authenticated
  using (bucket_id = 'setups' and name = (select auth.uid())::text || '/setup.json.gz')
  with check (bucket_id = 'setups' and name = (select auth.uid())::text || '/setup.json.gz');
drop policy if exists "own setup object: remove" on storage.objects;
create policy "own setup object: remove" on storage.objects
  for delete to authenticated
  using (bucket_id = 'setups' and name = (select auth.uid())::text || '/setup.json.gz');
