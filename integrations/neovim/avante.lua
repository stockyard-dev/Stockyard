-- ~/.config/nvim/lua/plugins/avante.lua
return {
  "yetone/avante.nvim",
  opts = {
    provider = "openai",
    openai = {
      endpoint = "http://localhost:4000/v1",
      model = "gpt-4o",
      api_key_name = "OPENAI_API_KEY",
    },
  },
}
