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

-- ---------------------------------------------------------------------------
-- Accountability partner (SPEC §4.15; ROADMAP 2.13). A user invites a
-- partner by email; the partner, signed in to their own account with that
-- email, accepts. The user's devices send reports, alerts and held
-- passwords as messages; the user asks for unlocks, a partner decides.
-- Email: a database webhook on partners (insert), partner_messages
-- (insert) and unlock_requests (insert) calls an Edge Function that emails
-- the partner(s) through the email provider (owner step, ROADMAP 2.13).

create table if not exists public.partners (
  id            uuid primary key default gen_random_uuid(),
  owner_id      uuid not null references auth.users (id) on delete cascade,
  owner_email   text not null,
  partner_email text not null check (length(partner_email) <= 320),
  partner_id    uuid references auth.users (id) on delete cascade,
  status        text not null default 'invited' check (status in ('invited', 'active')),
  created_at    timestamptz not null default now(),
  unique (owner_id, partner_email)
);
alter table public.partners enable row level security;

-- active_partner: whether the signed-in account partners owner.
create or replace function public.active_partner(owner uuid) returns boolean
  language sql stable security definer set search_path = public as $$
  select exists (select 1 from public.partners p
    where p.owner_id = owner and p.partner_id = (select auth.uid()) and p.status = 'active')
$$;

drop policy if exists "partners: read own and invitations" on public.partners;
create policy "partners: read own and invitations" on public.partners
  for select to authenticated
  using (owner_id = (select auth.uid()) or lower(partner_email) = lower((select auth.email())));
drop policy if exists "partners: invite" on public.partners;
create policy "partners: invite" on public.partners
  for insert to authenticated
  with check (owner_id = (select auth.uid()) and status = 'invited' and partner_id is null
    and owner_email = (select auth.email()));
drop policy if exists "partners: accept" on public.partners;
create policy "partners: accept" on public.partners
  for update to authenticated
  using (lower(partner_email) = lower((select auth.email())))
  with check (partner_id = (select auth.uid()) and status = 'active');
drop policy if exists "partners: remove" on public.partners;
create policy "partners: remove" on public.partners
  for delete to authenticated
  using (owner_id = (select auth.uid()) or lower(partner_email) = lower((select auth.email())));

create table if not exists public.partner_messages (
  id          bigint generated always as identity primary key,
  owner_id    uuid not null references auth.users (id) on delete cascade,
  owner_email text not null,
  kind        text not null check (kind in ('report', 'alert', 'password')),
  target      text not null default '' check (length(target) <= 300),
  subject     text not null check (length(subject) <= 300),
  body        text not null check (length(body) <= 20000),
  secret      text not null default '' check (length(secret) <= 200),
  created_at  timestamptz not null default now()
);
alter table public.partner_messages enable row level security;
drop policy if exists "messages: send" on public.partner_messages;
create policy "messages: send" on public.partner_messages
  for insert to authenticated
  with check (owner_id = (select auth.uid()) and owner_email = (select auth.email()));
-- A user reads what their devices sent, never a held password; a partner
-- reads everything the users they partner sent.
drop policy if exists "messages: read" on public.partner_messages;
create policy "messages: read" on public.partner_messages
  for select to authenticated
  using ((owner_id = (select auth.uid()) and kind <> 'password') or public.active_partner(owner_id));

create table if not exists public.unlock_requests (
  id          uuid primary key default gen_random_uuid(),
  owner_id    uuid not null references auth.users (id) on delete cascade,
  owner_email text not null,
  target      text not null check (length(target) <= 300),
  reason      text not null default '' check (length(reason) <= 2000),
  status      text not null default 'pending' check (status in ('pending', 'approved', 'denied', 'used')),
  secret      text not null default '' check (length(secret) <= 200),
  decided_by  text not null default '',
  created_at  timestamptz not null default now(),
  decided_at  timestamptz
);
alter table public.unlock_requests enable row level security;
drop policy if exists "requests: ask" on public.unlock_requests;
create policy "requests: ask" on public.unlock_requests
  for insert to authenticated
  with check (owner_id = (select auth.uid()) and status = 'pending' and secret = '');
drop policy if exists "requests: read" on public.unlock_requests;
create policy "requests: read" on public.unlock_requests
  for select to authenticated
  using (owner_id = (select auth.uid()) or public.active_partner(owner_id));
drop policy if exists "requests: decide" on public.unlock_requests;
create policy "requests: decide" on public.unlock_requests
  for update to authenticated
  using (public.active_partner(owner_id) and status = 'pending')
  with check (status in ('approved', 'denied'));
drop policy if exists "requests: use" on public.unlock_requests;
create policy "requests: use" on public.unlock_requests
  for update to authenticated
  using (owner_id = (select auth.uid()) and status = 'approved')
  with check (status = 'used' and secret = '');

-- Each device's last sign of life; a scheduled function (owner step)
-- alerts the partners of a device not heard from for a day.
create table if not exists public.heartbeats (
  owner_id uuid not null references auth.users (id) on delete cascade,
  device   text not null check (length(device) <= 200),
  seen_at  timestamptz not null,
  primary key (owner_id, device)
);
alter table public.heartbeats enable row level security;
drop policy if exists "heartbeats: own" on public.heartbeats;
create policy "heartbeats: own" on public.heartbeats
  for all to authenticated
  using (owner_id = (select auth.uid())) with check (owner_id = (select auth.uid()));
drop policy if exists "heartbeats: partners read" on public.heartbeats;
create policy "heartbeats: partners read" on public.heartbeats
  for select to authenticated using (public.active_partner(owner_id));
