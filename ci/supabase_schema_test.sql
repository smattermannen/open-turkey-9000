-- cloud/supabase/schema.sql in a real Postgres, as accounts reach it
-- through PostgREST: role authenticated, auth.uid()/auth.email() from the
-- JWT claims, Supabase's table grants, row-level security deciding. Run by
-- ci/supabase_schema_test.sh against ci/supabase_stub.sql; any failed
-- check raises. me invites anna; bob and eve are strangers (eve: an
-- account bob controls).
\set ON_ERROR_STOP 1

create function pg_temp.expect(ok boolean, what text) returns void language plpgsql as $$
begin
  if not coalesce(ok, false) then raise exception 'FAIL: %', what; end if;
  raise notice 'ok   %', what;
end $$;
-- refused: the statement raises, or changes no row (row-level security
-- filters an update or delete it doesn't allow).
create function pg_temp.refused(q text, what text) returns void language plpgsql as $$
declare n bigint;
begin
  execute q;
  get diagnostics n = row_count;
  if n > 0 then raise exception 'FAIL: % (% rows)', what, n; end if;
  raise notice 'ok   % (no row)', what;
exception when others then
  if sqlerrm like 'FAIL:%' then raise; end if;
  raise notice 'ok   % (%)', what, sqlerrm;
end $$;
create function pg_temp.as_user(n int) returns void language sql as $$
  select set_config('request.jwt.claim.sub', '00000000-0000-0000-0000-00000000000' || n, false),
         set_config('request.jwt.claim.email', (array['me', 'anna', 'bob', 'eve'])[n] || '@example.org', false)
$$;
grant execute on all functions in schema pg_temp to authenticated;
set role authenticated;

-- me invites anna; anna accepts.
select pg_temp.as_user(1);
insert into public.partners (owner_id, owner_email, partner_email)
  values (auth.uid(), auth.email(), 'anna@example.org');
select pg_temp.refused($$insert into public.partners (owner_id, owner_email, partner_email)
  values ('00000000-0000-0000-0000-000000000003', 'bob@example.org', 'me@example.org')$$, 'an invitation in another''s name');
select pg_temp.refused($$insert into public.partners (owner_id, owner_email, partner_email, status, partner_id)
  values (auth.uid(), auth.email(), 'eve@example.org', 'active', '00000000-0000-0000-0000-000000000004')$$, 'an invitation already accepted');
select pg_temp.as_user(3);
select pg_temp.refused($$update public.partners set partner_id = auth.uid(), status = 'active'$$, 'bob accepting anna''s invitation');
select pg_temp.as_user(2);
update public.partners set partner_id = auth.uid(), status = 'active' where partner_email = 'anna@example.org';
select pg_temp.expect((select count(*) from public.partners where status = 'active') = 1, 'anna accepted');

-- A held password: anna reads it, me never does, bob neither.
select pg_temp.as_user(1);
insert into public.partner_messages (owner_id, owner_email, kind, target, subject, body, secret)
  values (auth.uid(), auth.email(), 'password', 'lock:work', 'A password to hold', '', 's3cret-pw');
insert into public.partner_messages (owner_id, owner_email, kind, subject, body)
  values (auth.uid(), auth.email(), 'alert', 'Block switched off', '');
select pg_temp.expect((select count(*) from public.partner_messages where kind = 'password') = 0, 'me can''t read the held password back');
select pg_temp.expect((select count(*) from public.partner_messages) = 1, 'me reads its alert');
select pg_temp.as_user(2);
select pg_temp.expect((select secret from public.partner_messages where kind = 'password') = 's3cret-pw', 'anna reads the password she holds');
select pg_temp.as_user(3);
select pg_temp.expect((select count(*) from public.partner_messages) = 0, 'bob reads none of me''s messages');

-- bob invites eve (his own second account); accepting, eve must not make
-- herself me's partner by rewriting the row's owner.
select pg_temp.as_user(3);
insert into public.partners (owner_id, owner_email, partner_email) values (auth.uid(), auth.email(), 'eve@example.org');
select pg_temp.as_user(4);
select pg_temp.refused($$update public.partners set partner_id = auth.uid(), status = 'active',
  owner_id = '00000000-0000-0000-0000-000000000001', owner_email = 'me@example.org' where partner_email = 'eve@example.org'$$,
  'eve accepting as me''s partner by rewriting the owner');
select pg_temp.expect((select count(*) from public.partner_messages) = 0, 'eve reads none of me''s messages');
select pg_temp.refused($$update public.partners set partner_email = 'anna@example.org', partner_id = auth.uid(), status = 'active'
  where partner_email = 'eve@example.org'$$, 'eve changing the invited address');
update public.partners set partner_id = auth.uid(), status = 'active' where partner_email = 'eve@example.org';
select pg_temp.expect((select count(*) from public.partners where status = 'active') = 1, 'eve accepts bob''s invitation as it is');

-- An unlock request: anna decides, bob can't, me uses it once.
select pg_temp.as_user(1);
insert into public.unlock_requests (owner_id, owner_email, target, reason) values (auth.uid(), auth.email(), 'lock:work', 'done');
select pg_temp.refused($$insert into public.unlock_requests (owner_id, owner_email, target, status, secret)
  values (auth.uid(), auth.email(), 'lock:work', 'approved', 'x')$$, 'me approving its own request');
select pg_temp.as_user(3);
select pg_temp.refused($$update public.unlock_requests set status = 'approved', secret = 'x'$$, 'bob approving me''s request');
select pg_temp.as_user(2);
select pg_temp.refused($$update public.unlock_requests set status = 'approved', secret = 's3cret-pw',
  owner_id = '00000000-0000-0000-0000-000000000003'$$, 'anna moving the request to another owner');
select pg_temp.refused($$update public.unlock_requests set status = 'approved', secret = 's3cret-pw', target = 'settings-password'$$,
  'anna approving something else than asked');
update public.unlock_requests set status = 'approved', secret = 's3cret-pw', decided_by = auth.email(), decided_at = now();
select pg_temp.as_user(1);
select pg_temp.expect((select secret from public.unlock_requests) = 's3cret-pw', 'me reads the approval''s secret');
select pg_temp.refused($$update public.unlock_requests set status = 'pending'$$, 'me setting an approved request back');
update public.unlock_requests set status = 'used', secret = '';
select pg_temp.expect((select status from public.unlock_requests) = 'used', 'the request is used');

-- Heartbeats: own written, partners read, strangers not.
insert into public.heartbeats (owner_id, device, seen_at) values (auth.uid(), 'laptop', now());
select pg_temp.as_user(2);
select pg_temp.expect((select count(*) from public.heartbeats) = 1, 'anna sees me''s heartbeat');
select pg_temp.refused($$insert into public.heartbeats (owner_id, device, seen_at)
  values ('00000000-0000-0000-0000-000000000001', 'fake', now())$$, 'anna writing me''s heartbeat');
select pg_temp.as_user(3);
select pg_temp.expect((select count(*) from public.heartbeats) = 0, 'bob sees no heartbeat of me');

-- Setups: own row only.
select pg_temp.as_user(1);
insert into public.setups (user_id, uploaded_at, device, format, size) values (auth.uid(), now(), 'laptop', 1, 10);
select pg_temp.as_user(3);
select pg_temp.expect((select count(*) from public.setups) = 0, 'bob sees no setup of me');
select pg_temp.refused($$update public.setups set size = 0$$, 'bob changing me''s setup row');

-- Removal: anna leaves; she then reads nothing of me.
select pg_temp.as_user(2);
delete from public.partners where partner_email = 'anna@example.org';
select pg_temp.expect((select count(*) from public.partner_messages) = 0, 'anna, removed, reads nothing');
\echo schema: all passed
