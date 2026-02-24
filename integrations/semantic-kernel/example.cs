using Microsoft.SemanticKernel;

var builder = Kernel.CreateBuilder();
builder.AddOpenAIChatCompletion(
    modelId: "gpt-4o",
    endpoint: new Uri("http://localhost:4000/v1"),
    apiKey: "any-string"
);
var kernel = builder.Build();
