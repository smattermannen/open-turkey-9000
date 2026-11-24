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

-- ---------------------------------------------------------------------
-- Accountability partner (SPEC §4.15; ROADMAP 2.13). The tables are closed
-- to accounts (row-level security on, no policies): everything goes
-- through the functions below (POST /rest/v1/rpc/<name>), each security
-- definer and each checking who calls. The owner is the KC9K user; a
-- partner is invited by email and accepts signed in with that address.
-- Email: messages are queued here; the sender (cmd/kc9k-mailer, service
-- role key) takes and marks them (outbox_take, outbox_done).

create table if not exists public.partners (
  id          uuid primary key default gen_random_uuid(),
  owner       uuid not null references auth.users (id) on delete cascade,
  email       text not null check (position('@' in email) > 1 and length(email) <= 320),
  partner     uuid references auth.users (id) on delete set null,
  accepted    boolean not null default false,
  reports     text not null default 'weekly' check (reports in ('daily', 'weekly', 'off')),
  alerts      boolean not null default true,
  removed     boolean not null default false,
  created_at  timestamptz not null default now()
);
create unique index if not exists partners_one_each on public.partners (owner, email) where not removed;

create table if not exists public.partner_held (
  partner_id  uuid not null references public.partners (id) on delete cascade,
  what        text not null check (length(what) <= 300),
  secret      text not null check (length(secret) <= 300),
  primary key (partner_id, what)
);

create table if not exists public.partner_messages (
  id          bigint generated always as identity primary key,
  partner_id  uuid not null references public.partners (id) on delete cascade,
  kind        text not null check (kind in ('invite', 'report', 'alert', 'unlock-request', 'removed')),
  subject     text not null check (length(subject) <= 300),
  body        text not null default '' check (length(body) <= 20000),
  created_at  timestamptz not null default now(),
  emailed_at  timestamptz
);
create index if not exists partner_messages_unsent on public.partner_messages (id) where emailed_at is null;

create table if not exists public.unlock_requests (
  id          bigint generated always as identity primary key,
  partner_id  uuid not null references public.partners (id) on delete cascade,
  what        text not null,
  reason      text not null default '' check (length(reason) <= 2000),
  status      text not null default 'pending' check (status in ('pending', 'approved', 'denied', 'used')),
  secret      text,
  created_at  timestamptz not null default now(),
  answered_at timestamptz
);

create table if not exists public.heartbeats (
  owner       uuid primary key references auth.users (id) on delete cascade,
  device      text not null default '' check (length(device) <= 200),
  seen_at     timestamptz not null default now()
);

alter table public.partners enable row level security;
alter table public.partner_held enable row level security;
alter table public.partner_messages enable row level security;
alter table public.unlock_requests enable row level security;
alter table public.heartbeats enable row level security;

create or replace function public.my_email() returns text language sql stable security definer
set search_path = '' as $$ select lower(email) from auth.users where id = auth.uid() $$;
revoke all on function public.my_email() from public, anon;

create or replace function public.partner_json(p public.partners) returns jsonb language sql stable security definer
set search_path = '' as $$
  select jsonb_build_object('id', p.id, 'email', p.email, 'accepted', p.accepted, 'reports', p.reports,
    'alerts', p.alerts, 'created_at', p.created_at,
    'holds', coalesce((select jsonb_agg(h.what order by h.what) from public.partner_held h where h.partner_id = p.id), '[]'::jsonb))
$$;
revoke all on function public.partner_json(public.partners) from public, anon, authenticated;

create or replace function public.partner_invite(p_email text, p_reports text, p_alerts boolean)
returns jsonb language plpgsql security definer set search_path = '' as $$
declare p public.partners; e text := lower(trim(p_email));
begin
  if auth.uid() is null then raise exception 'not signed in'; end if;
  if position('@' in e) < 2 or e = public.my_email() then raise exception 'a partner is someone else''s email address'; end if;
  if exists (select 1 from public.partners where owner = auth.uid() and email = e and not removed) then
    raise exception 'already a partner'; end if;
  insert into public.partners (owner, email, reports, alerts) values (auth.uid(), e, p_reports, p_alerts) returning * into p;
  insert into public.partner_messages (partner_id, kind, subject, body) values (p.id, 'invite',
    public.my_email() || ' asks you to be their accountability partner',
    public.my_email() || ' uses Killer Chicken 9000 and asks you to be their accountability partner. Sign in to KC9K with this address to accept.');
  return public.partner_json(p);
end $$;

create or replace function public.partner_list() returns jsonb language sql stable security definer
set search_path = '' as $$
  select coalesce(jsonb_agg(public.partner_json(p) order by p.created_at), '[]'::jsonb)
  from public.partners p where p.owner = auth.uid() and not p.removed
$$;

create or replace function public.partner_set(p_id uuid, p_reports text, p_alerts boolean)
returns void language plpgsql security definer set search_path = '' as $$
begin
  update public.partners set reports = p_reports, alerts = p_alerts where id = p_id and owner = auth.uid() and not removed;
  if not found then raise exception 'no such partner' using errcode = 'P0002'; end if;
end $$;

create or replace function public.partner_remove(p_id uuid) returns void language plpgsql security definer
set search_path = '' as $$
begin
  update public.partners set removed = true where id = p_id and owner = auth.uid() and not removed;
  if not found then raise exception 'no such partner' using errcode = 'P0002'; end if;
  delete from public.partner_held where partner_id = p_id;
  update public.unlock_requests set status = 'denied' where partner_id = p_id and status = 'pending';
  insert into public.partner_messages (partner_id, kind, subject) values (p_id, 'removed',
    public.my_email() || ' removed you as their accountability partner');
end $$;

create or replace function public.partner_send(p_kind text, p_period text, p_subject text, p_body text)
returns integer language plpgsql security definer set search_path = '' as $$
declare n integer;
begin
  if p_kind not in ('report', 'alert') then raise exception 'only reports and alerts'; end if;
  insert into public.partner_messages (partner_id, kind, subject, body)
    select id, p_kind, p_subject, p_body from public.partners
    where owner = auth.uid() and not removed and accepted
      and ((p_kind = 'alert' and alerts) or (p_kind = 'report' and reports = p_period));
  get diagnostics n = row_count;
  return n;
end $$;

create or replace function public.partner_hold(p_id uuid, p_what text, p_secret text)
returns void language plpgsql security definer set search_path = '' as $$
begin
  if not exists (select 1 from public.partners where id = p_id and owner = auth.uid() and accepted and not removed) then
    raise exception 'no such partner, or not accepted yet' using errcode = 'P0002'; end if;
  delete from public.partner_held h using public.partners p
    where h.partner_id = p.id and p.owner = auth.uid() and h.what = p_what;
  insert into public.partner_held (partner_id, what, secret) values (p_id, p_what, p_secret);
end $$;

create or replace function public.partner_release(p_what text) returns void language sql security definer
set search_path = '' as $$
  delete from public.partner_held h using public.partners p
  where h.partner_id = p.id and p.owner = auth.uid() and h.what = p_what
$$;

create or replace function public.unlock_request(p_what text, p_reason text)
returns jsonb language plpgsql security definer set search_path = '' as $$
declare out jsonb := '[]'::jsonb; r record;
begin
  for r in
    with ins as (
      insert into public.unlock_requests (partner_id, what, reason)
        select h.partner_id, h.what, p_reason from public.partner_held h join public.partners p on p.id = h.partner_id
        where p.owner = auth.uid() and not p.removed and h.what = p_what
      returning id, partner_id, what, reason, status, created_at)
    select ins.*, p.email as partner_email from ins join public.partners p on p.id = ins.partner_id
  loop
    insert into public.partner_messages (partner_id, kind, subject, body) values (r.partner_id, 'unlock-request',
      public.my_email() || ' asks you to approve unlocking ' || r.what, r.reason);
    out := out || jsonb_build_object('id', r.id, 'partner', r.partner_email, 'what', r.what, 'reason', r.reason,
      'status', r.status, 'created_at', r.created_at);
  end loop;
  if jsonb_array_length(out) = 0 then raise exception 'no partner holds that password' using errcode = 'P0002'; end if;
  return out;
end $$;

create or replace function public.unlock_status() returns jsonb language sql stable security definer
set search_path = '' as $$
  select coalesce(jsonb_agg(jsonb_build_object('id', r.id, 'partner', p.email, 'what', r.what, 'reason', r.reason,
    'status', r.status, 'created_at', r.created_at, 'answered_at', r.answered_at,
    'secret', case when r.status = 'approved' then r.secret end) order by r.id), '[]'::jsonb)
  from public.unlock_requests r join public.partners p on p.id = r.partner_id where p.owner = auth.uid()
$$;

create or replace function public.unlock_used(p_id bigint) returns void language sql security definer
set search_path = '' as $$
  update public.unlock_requests r set status = 'used', secret = null from public.partners p
  where r.id = p_id and p.id = r.partner_id and p.owner = auth.uid()
$$;

create or replace function public.heartbeat(p_device text) returns void language sql security definer
set search_path = '' as $$
  insert into public.heartbeats (owner, device, seen_at) values (auth.uid(), p_device, now())
  on conflict (owner) do update set device = excluded.device, seen_at = excluded.seen_at
$$;

create or replace function public.partner_accept(p_id uuid) returns void language plpgsql security definer
set search_path = '' as $$
begin
  update public.partners set accepted = true, partner = auth.uid()
    where id = p_id and email = public.my_email() and owner <> auth.uid() and not removed;
  if not found then raise exception 'no such invitation' using errcode = 'P0002'; end if;
end $$;

create or replace function public.partner_inbox() returns jsonb language sql stable security definer
set search_path = '' as $$
  select jsonb_build_object(
    'invitations', coalesce((select jsonb_agg(jsonb_build_object('id', p.id, 'from', u.email))
      from public.partners p join auth.users u on u.id = p.owner
      where p.email = public.my_email() and not p.accepted and not p.removed), '[]'::jsonb),
    'watching', coalesce((select jsonb_agg(jsonb_build_object('email', u.email, 'device', hb.device, 'last_seen', hb.seen_at))
      from public.partners p join auth.users u on u.id = p.owner left join public.heartbeats hb on hb.owner = p.owner
      where p.partner = auth.uid() and p.accepted and not p.removed), '[]'::jsonb),
    'held', coalesce((select jsonb_agg(jsonb_build_object('from', u.email, 'what', h.what, 'secret', h.secret))
      from public.partner_held h join public.partners p on p.id = h.partner_id join auth.users u on u.id = p.owner
      where p.partner = auth.uid() and not p.removed), '[]'::jsonb),
    'messages', coalesce((select jsonb_agg(jsonb_build_object('id', m.id, 'from', u.email, 'kind', m.kind,
        'subject', m.subject, 'body', m.body, 'created_at', m.created_at) order by m.id desc)
      from public.partner_messages m join public.partners p on p.id = m.partner_id join auth.users u on u.id = p.owner
      where p.partner = auth.uid() and m.kind <> 'invite' and m.created_at > now() - interval '60 days'), '[]'::jsonb),
    'requests', coalesce((select jsonb_agg(jsonb_build_object('id', r.id, 'from', u.email, 'what', r.what,
        'reason', r.reason, 'status', r.status, 'created_at', r.created_at) order by r.id)
      from public.unlock_requests r join public.partners p on p.id = r.partner_id join auth.users u on u.id = p.owner
      where p.partner = auth.uid() and not p.removed and r.status = 'pending'), '[]'::jsonb))
$$;

create or replace function public.unlock_answer(p_id bigint, p_approve boolean) returns void language plpgsql
security definer set search_path = '' as $$
begin
  update public.unlock_requests r set status = case when p_approve then 'approved' else 'denied' end,
      answered_at = now(),
      secret = case when p_approve then (select h.secret from public.partner_held h where h.partner_id = r.partner_id and h.what = r.what) end
    from public.partners p
    where r.id = p_id and p.id = r.partner_id and p.partner = auth.uid() and not p.removed and r.status = 'pending';
  if not found then raise exception 'no such request' using errcode = 'P0002'; end if;
end $$;

-- The mail sender's two functions: the service role only.
create or replace function public.outbox_take(p_limit integer) returns jsonb language sql stable security definer
set search_path = '' as $$
  select coalesce(jsonb_agg(jsonb_build_object('id', m.id, 'to', p.email, 'subject', m.subject, 'body', m.body) order by m.id), '[]'::jsonb)
  from (select * from public.partner_messages where emailed_at is null order by id limit p_limit) m
  join public.partners p on p.id = m.partner_id
$$;
create or replace function public.outbox_done(p_ids bigint[]) returns void language sql security definer
set search_path = '' as $$
  update public.partner_messages set emailed_at = now() where id = any(p_ids)
$$;

revoke all on function public.outbox_take(integer), public.outbox_done(bigint[]) from public, anon, authenticated;
grant execute on function public.outbox_take(integer), public.outbox_done(bigint[]) to service_role;
revoke all on function public.partner_invite(text, text, boolean), public.partner_list(), public.partner_set(uuid, text, boolean),
  public.partner_remove(uuid), public.partner_send(text, text, text, text), public.partner_hold(uuid, text, text),
  public.partner_release(text), public.unlock_request(text, text), public.unlock_status(), public.unlock_used(bigint),
  public.heartbeat(text), public.partner_accept(uuid), public.partner_inbox(), public.unlock_answer(bigint, boolean)
  from public, anon;
grant execute on function public.partner_invite(text, text, boolean), public.partner_list(), public.partner_set(uuid, text, boolean),
  public.partner_remove(uuid), public.partner_send(text, text, text, text), public.partner_hold(uuid, text, text),
  public.partner_release(text), public.unlock_request(text, text), public.unlock_status(), public.unlock_used(bigint),
  public.heartbeat(text), public.partner_accept(uuid), public.partner_inbox(), public.unlock_answer(bigint, boolean)
  to authenticated;
