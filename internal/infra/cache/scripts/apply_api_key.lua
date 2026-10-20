local api_key = KEYS[1]
local key_id = ARGV[1]
local tenant_id = ARGV[2]
local hash = ARGV[3]
local status = ARGV[4]

local versionRaw = ARGV[5]
local version = tonumber(versionRaw)
if versionRaw and not version then
	return redis.error_reply("invalid version")
end

local ttlRaw = ARGV[6] -- ttl in seconds
local ttl = tonumber(ttlRaw)

if ttlRaw and not ttl then
	return redis.error_reply("invalid ttl")
end

local currentVersionRaw = redis.call("HGET", api_key, "version")
local currentVersion = tonumber(currentVersionRaw)
if currentVersionRaw and not currentVersion then
	return redis.error_reply("invalid_cached_version")
end

if currentVersion then
	if currentVersion > version then
		return -1
	end
	if currentVersion == version then
		return 0
	end
end

redis.call(
	"HSET",
	api_key,
	"key_id",
	key_id,
	"tenant_id",
	tenant_id,
	"hash",
	hash,
	"status",
	status,
	"version",
	version
)

if ttl > 0 then
	redis.call("EXPIRE", api_key, ttl)
end
return 1
