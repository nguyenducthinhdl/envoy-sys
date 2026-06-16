package snapshot

// luaAuthScript rejects every request with HTTP 401 Unauthorized.
const luaAuthScript = `
function envoy_on_request(request_handle)
  request_handle:respond(
    {[":status"] = "401", ["content-type"] = "text/plain"},
    "Unauthorized"
  )
end
`
