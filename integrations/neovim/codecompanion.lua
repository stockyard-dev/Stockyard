-- For codecompanion.nvim
require("codecompanion").setup({
  adapters = {
    openai = function()
      return require("codecompanion.adapters").extend("openai", {
        url = "http://localhost:4000/v1/chat/completions",
      })
    end,
  },
})
