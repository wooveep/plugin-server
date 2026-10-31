package mediaexecution

// DispatchConsumeScript consumes an original Console-installed media ticket.
// Keys are ticket, balance control, account, optional concrete Key mapping, and
// the existing execution Stream. Arguments are the immutable ticket digest and
// original media_dispatch event. No missing ticket is created here; consumed
// tickets remain retained across gateway restarts. The Console must never mint
// a replacement ticket after an original handoff becomes uncertain.
const DispatchConsumeScript = `
local function deny(reason) return {'deny',reason} end
if redis.call('TYPE',KEYS[1]).ok ~= 'hash' then return deny('media_ticket_missing') end
local ticket=redis.call('HMGET',KEYS[1],'state','digest','expires_at_ms','mode','generation','account_id','admission_revision','mapping_revision','credential_generation','route_id','key_id','operation_id')
if ticket[2] ~= ARGV[1] then return deny('media_ticket_conflict') end
if ticket[1] ~= 'prepared' then return deny('media_ticket_consumed') end
local clock=redis.call('TIME')
local now=tonumber(clock[1])*1000+math.floor(tonumber(clock[2])/1000)
local expiry=tonumber(ticket[3])
if not expiry or now >= expiry then return deny('media_ticket_expired') end
local valid,event=pcall(cjson.decode,ARGV[2])
if not valid or type(event) ~= 'table' or event.operation_id ~= ticket[12] or event.ticket_digest ~= ticket[2] then return deny('media_event_conflict') end
local function record(decision,reason)
  local stream_type=redis.call('TYPE',KEYS[5]).ok
  if stream_type ~= 'none' and stream_type ~= 'stream' then return nil end
  event.decision=decision
  event.reason_code=reason or cjson.null
  event.recorded_at_ms=string.format('%.0f',now)
  local wire=cjson.encode(event)
  local id=redis.call('XADD',KEYS[5],'*','message_type','media_dispatch','event',wire)
  redis.call('HSET',KEYS[1],'state',decision,'event',wire,'stream_id',id)
  return id
end
local function rejected(reason)
  if not record('denied',reason) then return deny('media_stream_unavailable') end
  return deny(reason)
end
if ticket[4] == 'create' then
  if redis.call('TYPE',KEYS[2]).ok ~= 'hash' or redis.call('TYPE',KEYS[3]).ok ~= 'hash' then return deny('projection_unready') end
  local control=redis.call('HMGET',KEYS[2],'cache_generation','projection_schema','projection_state')
  if control[1] ~= ticket[5] or control[2] ~= 'balance.v1' or control[3] ~= 'ready' then return deny('projection_unready') end
  local account=redis.call('HMGET',KEYS[3],'account_id','cache_generation','projection_schema','projection_state','admission_state_revision','mapping_revision','credential_generation','amount','currency','scale')
  if account[1] ~= ticket[6] or account[2] ~= ticket[5] or account[3] ~= 'balance.v1' or account[4] ~= 'ready' then return deny('projection_unready') end
  if account[5] ~= ticket[7] or account[6] ~= ticket[8] or account[7] ~= ticket[9] then return rejected('authorization_stale') end
  if account[9] ~= 'CNY' or account[10] ~= '10' or type(account[8]) ~= 'string' then return deny('projection_unready') end
  if account[8] == '0' or string.sub(account[8],1,1) == '-' then return rejected('insufficient_balance') end
  if not string.match(account[8],'^[1-9][0-9]*$') or #account[8] > 19 or (#account[8] == 19 and account[8] > '9223372036854775807') then return deny('projection_unready') end
  if ticket[11] and ticket[11] ~= '' then
    if redis.call('TYPE',KEYS[4]).ok ~= 'hash' then return rejected('authorization_stale') end
    local mapping=redis.call('HMGET',KEYS[4],'account_id','cache_generation','projection_schema','projection_state','admission_state_revision','mapping_revision','credential_generation','enabled','authorized_routes')
    if mapping[1] ~= account[1] or mapping[2] ~= account[2] or mapping[3] ~= 'balance.v1' or mapping[4] ~= 'ready' or mapping[5] ~= account[5] or mapping[6] ~= account[6] or mapping[7] ~= account[7] or mapping[8] ~= '1' then return rejected('authorization_stale') end
    if type(mapping[9]) ~= 'string' or string.sub(mapping[9],1,1) ~= '[' then return deny('projection_unready') end
    local ok,routes=pcall(cjson.decode,mapping[9])
    if not ok or type(routes) ~= 'table' then return deny('projection_unready') end
    local allowed=false
    for _,route in ipairs(routes) do
      if type(route) ~= 'string' then return deny('projection_unready') end
      if route == ticket[10] then allowed=true end
    end
    if not allowed then return rejected('route_authorization_missing') end
  end
elseif ticket[4] ~= 'query' then return deny('media_ticket_invalid') end
local stream_id=record('consumed',nil)
if not stream_id then return deny('media_stream_unavailable') end
return {'allow',stream_id}
`
