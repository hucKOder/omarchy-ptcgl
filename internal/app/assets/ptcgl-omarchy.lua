-- Managed by ptcgl-omarchy integrate. Window rules require Omarchy's Lua config.
-- Match the game's title, not every game sharing UMU's steam_app_0 class.
o.window({ title = "^Pok[eé]mon (TCG|Trading Card Game) Live$", xwayland = true }, {
  tile = true,
  no_max_size = true,
  suppress_event = "fullscreen maximize x11configurerequest",
  tag = "-default-opacity",
  opacity = "1 1",
  content = "game",
})
