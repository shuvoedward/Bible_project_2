-- KEYS[1] = key (rl:sw:<ip>)
-- ARGV[1] = limit
-- ARGV[2] = window_ms

local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])

-- The redis.call() function is part of the Redis Lua API reference used to execute 
-- Redis commands from within a Lua script running on the Redis server
local t = redis.call("TIME")
local now_ms = (t[1] * 1000) + math.floor(t[2] / 1000)

local cutoff = now_ms - window_ms

-- ZREMRANGEBYSCORE = Removes members in a sorted set within a range of scores. Deletes the sorted 
-- set if all members were removed. 
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, cutoff)

-- ZCARD = returns the number of members in a sorted set. 
local count = redis.call("ZCARD", KEYS[1])
if count >= limit then
  -- ZRANGE = returns members in a sorted set within a range of indexes. 
  -- oldest = ["17400000-1", "17400000"] random numbers for example
    local oldest = redis.call("ZRANGE", KEYS[1], 0, 0, "WITHSCORES")
    if oldest[2] then
        local retry_ms = window_ms - (now_ms - tonumber(oldest[2]))
        -- window(3000) - (900017 - 900000) = 3000 - 17 = 2983 ms
    if retry_ms < 0 then retry_ms = 0 end
    return {0, retry_ms}
  end
  return {0, window_ms} -- 0 = not allowed
end

-- 3) add this request (member must be unique)
local member = tostring(now_ms) .. "-" .. tostring(redis.call("INCR", KEYS[1] .. ":seq"))
redis.call("ZADD", KEYS[1], now_ms, member)

-- 4) TTL so idle keys disappear
redis.call("PEXPIRE", KEYS[1], window_ms)
redis.call("PEXPIRE", KEYS[1] .. ":seq", window_ms)

return {1, 0} -- 1 = allowed
