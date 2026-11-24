-- cloud/supabase/schema.sql's partner functions in a real Postgres, as the
-- accounts call them (role authenticated, auth.uid() from the JWT claim),
-- against ci/supabase_stub.sql's stand-ins for Supabase's auth and storage.
-- The same story as internal/cloud/partners_test.go (the Go fake). Run by
-- ci/supabase_schema_test.sh; any failed check raises.
\set ON_ERROR_STOP 1
\set me   '''00000000-0000-0000-0000-000000000001'''
\set anna '''00000000-0000-0000-0000-000000000002'''
\set bob  '''00000000-0000-0000-0000-000000000003'''

create function pg_temp.expect(ok boolean, what text) returns void language plpgsql as $$
begin
  if not ok then raise exception 'FAIL: %', what; end if;
  raise notice 'ok   %', what;
end $$;
create function pg_temp.refused(q text, what text) returns void language plpgsql as $$
begin
  execute q;
  raise exception 'FAIL: %', what;
exception when others then
  if sqlerrm like 'FAIL:%' then raise; end if;
  raise notice 'ok   % (%)', what, sqlerrm;
end $$;
grant execute on all functions in schema pg_temp to authenticated;

set role authenticated;
select set_config('request.jwt.claim.sub', :me, false);
select pg_temp.refused($$select public.partner_invite('me@example.org', 'daily', true)$$, 'inviting oneself');
select public.partner_invite('Anna@example.org', 'daily', true) ->> 'id' as pa \gset
select public.partner_invite('bob@example.org', 'weekly', false) ->> 'id' as pb \gset
select pg_temp.refused($$select public.partner_invite('anna@example.org', 'daily', true)$$, 'the same partner twice');
select pg_temp.refused($$select * from public.partners$$, 'reading the partners table directly');
select pg_temp.refused($$select * from public.partner_held$$, 'reading held passwords directly');
select pg_temp.refused($$select public.outbox_take(10)$$, 'an account taking the mail queue');
select pg_temp.refused(format($$select public.partner_hold(%L, 'settings-password', 'x')$$, :'pa'), 'holding before acceptance');

select set_config('request.jwt.claim.sub', :bob, false);
select pg_temp.refused(format($$select public.partner_accept(%L)$$, :'pa'), 'bob accepting anna''s invitation');
select public.partner_accept(:'pb');
select set_config('request.jwt.claim.sub', :anna, false);
select pg_temp.expect(jsonb_array_length(public.partner_inbox() -> 'invitations') = 1, 'anna sees her invitation');
select public.partner_accept(:'pa');

select set_config('request.jwt.claim.sub', :me, false);
select pg_temp.expect(public.partner_send('alert', '', 'Block switched off', '') = 1, 'an alert to anna only');
select pg_temp.expect(public.partner_send('report', 'weekly', 'Your week', '') = 1, 'a weekly report to bob only');
select public.partner_hold(:'pa', 'lock:work', 's3cret-pw');
select public.heartbeat('laptop');
select pg_temp.expect(public.partner_list() -> 0 -> 'holds' = '["lock:work"]'::jsonb, 'the list says what anna holds');
select pg_temp.refused($$select public.unlock_request('lock:other', 'please')$$, 'a request for what nobody holds');
select (public.unlock_request('lock:work', 'the exam is over') -> 0 ->> 'id')::bigint as req \gset
select pg_temp.expect(public.unlock_status() -> 0 ->> 'secret' is null, 'no secret before the answer');

select set_config('request.jwt.claim.sub', :bob, false);
select pg_temp.expect(jsonb_array_length(public.partner_inbox() -> 'held') = 0, 'bob holds nothing');
select pg_temp.refused(format($$select public.unlock_answer(%s, true)$$, :req), 'bob approving anna''s request');
select set_config('request.jwt.claim.sub', :anna, false);
select pg_temp.expect(public.partner_inbox() -> 'held' -> 0 ->> 'secret' = 's3cret-pw', 'anna reads the password she holds');
select pg_temp.expect(public.partner_inbox() -> 'watching' -> 0 ->> 'device' = 'laptop', 'anna sees the heartbeat');
select pg_temp.expect(jsonb_array_length(public.partner_inbox() -> 'messages') = 2, 'anna''s messages: the alert and the request');
select public.unlock_answer(:req, true);

select set_config('request.jwt.claim.sub', :me, false);
select pg_temp.expect(public.unlock_status() -> 0 ->> 'secret' = 's3cret-pw', 'the approved request gives the secret');
select public.unlock_used(:req);
select pg_temp.expect(public.unlock_status() -> 0 ->> 'status' = 'used' and public.unlock_status() -> 0 ->> 'secret' is null,
  'a used request keeps no secret');
select public.partner_remove(:'pa');
select set_config('request.jwt.claim.sub', :anna, false);
select pg_temp.expect(jsonb_array_length(public.partner_inbox() -> 'held') = 0
  and jsonb_array_length(public.partner_inbox() -> 'watching') = 0, 'a removed partner holds and watches nothing');

reset role;
set role service_role;
select pg_temp.expect(jsonb_array_length(public.outbox_take(100)) = 6, 'the sender sees every queued email (2 invites, alert, report, request, removal)');
select public.outbox_done(array(select (e ->> 'id')::bigint from jsonb_array_elements(public.outbox_take(100)) e));
select pg_temp.expect(jsonb_array_length(public.outbox_take(100)) = 0, 'sent ones are not taken again');
\echo partner functions: all passed
