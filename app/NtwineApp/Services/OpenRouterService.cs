using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Threading.Tasks;

namespace NtwineApp.Services;

public class OpenRouterService
{
    private static readonly HttpClient Http = new();
    private List<OpenRouterModel> _cachedModels = new();

    public async Task<List<OpenRouterModel>> FetchModels()
    {
        try
        {
            var resp = await Http.GetStringAsync("https://openrouter.ai/api/v1/models");
            var result = JsonSerializer.Deserialize<ModelsResponse>(resp);
            if (result?.Data != null)
            {
                _cachedModels = result.Data
                    .Where(m => m.Id != null)
                    .OrderBy(m => m.Pricing?.Prompt == "0" && m.Pricing?.Completion == "0" ? 0 : 1)
                    .ThenBy(m => m.Name)
                    .ToList();
            }
        }
        catch
        {
            // use cache if fetch fails
        }
        return _cachedModels;
    }

    public List<OpenRouterModel> GetCached() => _cachedModels;

    public List<string> GetProviders()
    {
        return _cachedModels
            .Select(m => ExtractProvider(m.Id))
            .Where(p => !string.IsNullOrEmpty(p))
            .Distinct()
            .OrderBy(p => p)
            .ToList();
    }

    public List<string> GetCategories()
    {
        return new List<string>
        {
            "All",
            "Free",
            "Programming",
            "Roleplay",
            "Marketing",
            "SEO",
            "Technology",
            "Science",
            "Translation",
            "Legal",
            "Finance",
            "Health",
            "Trivia",
            "Academia"
        };
    }

    public List<OpenRouterModel> FilterByProvider(string provider)
    {
        if (string.IsNullOrEmpty(provider) || provider == "All")
            return _cachedModels;

        return _cachedModels
            .Where(m => ExtractProvider(m.Id) == provider)
            .ToList();
    }

    public List<OpenRouterModel> FilterByCategory(string category)
    {
        if (string.IsNullOrEmpty(category) || category == "All")
            return _cachedModels;

        if (category == "Free")
            return _cachedModels.Where(m => m.IsFree).ToList();

        // openrouter doesn't expose categories in the API directly
        // but we can filter by known model tags/descriptions
        return _cachedModels
            .Where(m => MatchesCategory(m, category))
            .ToList();
    }

    public List<OpenRouterModel> Search(string query)
    {
        if (string.IsNullOrWhiteSpace(query))
            return _cachedModels;

        var lower = query.ToLowerInvariant();
        return _cachedModels
            .Where(m =>
                (m.Name?.ToLowerInvariant().Contains(lower) ?? false) ||
                (m.Id?.ToLowerInvariant().Contains(lower) ?? false) ||
                (m.Description?.ToLowerInvariant().Contains(lower) ?? false))
            .ToList();
    }

    public static string ExtractProvider(string? modelId)
    {
        if (modelId == null) return "";
        var slash = modelId.IndexOf('/');
        return slash > 0 ? modelId[..slash] : "";
    }

    public static string ExtractModelName(string? modelId)
    {
        if (modelId == null) return "";
        var slash = modelId.IndexOf('/');
        return slash > 0 ? modelId[(slash + 1)..] : modelId;
    }

    public static string FormatProviderName(string provider)
    {
        return provider switch
        {
            "anthropic" => "Anthropic",
            "openai" => "OpenAI",
            "google" => "Google",
            "deepseek" => "DeepSeek",
            "meta-llama" => "Meta",
            "mistralai" => "Mistral",
            "qwen" => "Qwen",
            "x-ai" => "xAI",
            "cohere" => "Cohere",
            "nousresearch" => "Nous",
            "microsoft" => "Microsoft",
            "amazon" => "Amazon",
            "minimax" => "MiniMax",
            "moonshotai" => "Moonshot",
            _ => provider
        };
    }

    public static string GetCostLabel(OpenRouterModel model)
    {
        if (model.IsFree) return "free";

        var promptCost = ParseCost(model.Pricing?.Prompt);
        if (promptCost <= 0) return "free";
        if (promptCost <= 0.5) return "$";
        if (promptCost <= 2.0) return "$$";
        if (promptCost <= 5.0) return "$$$";
        return "$$$$";
    }

    public static string GetCostPerMillion(OpenRouterModel model)
    {
        var input = ParseCost(model.Pricing?.Prompt);
        var output = ParseCost(model.Pricing?.Completion);
        if (input <= 0 && output <= 0) return "free";
        return $"${input:F2}/{output:F2} per 1M tokens";
    }

    public static string GetAgentColor(int index)
    {
        var colors = new[]
        {
            "#8b5cf6", "#3b82f6", "#10b981", "#ec4899",
            "#f59e0b", "#ef4444", "#06b6d4", "#84cc16",
            "#a855f7", "#f97316"
        };
        return colors[index % colors.Length];
    }

    private static double ParseCost(string? cost)
    {
        if (string.IsNullOrEmpty(cost)) return 0;
        if (double.TryParse(cost, out var val)) return val * 1_000_000;
        return 0;
    }

    private static bool MatchesCategory(OpenRouterModel model, string category)
    {
        var desc = (model.Description ?? "").ToLowerInvariant();
        var name = (model.Name ?? "").ToLowerInvariant();

        return category.ToLowerInvariant() switch
        {
            "programming" => desc.Contains("code") || desc.Contains("program") || desc.Contains("develop") ||
                            name.Contains("code") || name.Contains("coder") || name.Contains("devstral"),
            "roleplay" => desc.Contains("roleplay") || desc.Contains("creative") || desc.Contains("story"),
            "science" => desc.Contains("science") || desc.Contains("research") || desc.Contains("math"),
            "translation" => desc.Contains("translat") || desc.Contains("multilingual"),
            "finance" => desc.Contains("financ") || desc.Contains("trading"),
            "health" => desc.Contains("health") || desc.Contains("medical") || desc.Contains("clinical"),
            "academia" => desc.Contains("academic") || desc.Contains("scholar") || desc.Contains("research"),
            "legal" => desc.Contains("legal") || desc.Contains("law"),
            "marketing" => desc.Contains("marketing") || desc.Contains("copywriting"),
            "seo" => desc.Contains("seo") || desc.Contains("search engine"),
            "technology" => true,
            _ => true
        };
    }
}

public class ModelsResponse
{
    [JsonPropertyName("data")]
    public List<OpenRouterModel>? Data { get; set; }
}

public class OpenRouterModel
{
    [JsonPropertyName("id")]
    public string? Id { get; set; }

    [JsonPropertyName("name")]
    public string? Name { get; set; }

    [JsonPropertyName("description")]
    public string? Description { get; set; }

    [JsonPropertyName("pricing")]
    public ModelPricing? Pricing { get; set; }

    [JsonPropertyName("context_length")]
    public int ContextLength { get; set; }

    [JsonPropertyName("top_provider")]
    public TopProvider? TopProvider { get; set; }

    [JsonPropertyName("architecture")]
    public ModelArchitecture? Architecture { get; set; }

    public bool IsFree => (Pricing?.Prompt == "0" || Pricing?.Prompt == null) &&
                          (Pricing?.Completion == "0" || Pricing?.Completion == null);

    public string Provider => OpenRouterService.ExtractProvider(Id);
    public string ShortName => OpenRouterService.ExtractModelName(Id);
    public string CostLabel => OpenRouterService.GetCostLabel(this);
    public string CostDetail => OpenRouterService.GetCostPerMillion(this);

    public bool SupportsTools => Architecture?.Modality?.Contains("text") ?? true;
    public bool SupportsVision => Architecture?.Modality?.Contains("image") ?? false;
}

public class ModelPricing
{
    [JsonPropertyName("prompt")]
    public string? Prompt { get; set; }

    [JsonPropertyName("completion")]
    public string? Completion { get; set; }

    [JsonPropertyName("image")]
    public string? Image { get; set; }
}

public class TopProvider
{
    [JsonPropertyName("max_completion_tokens")]
    public int? MaxCompletionTokens { get; set; }

    [JsonPropertyName("is_moderated")]
    public bool IsModerated { get; set; }
}

public class ModelArchitecture
{
    [JsonPropertyName("modality")]
    public string? Modality { get; set; }

    [JsonPropertyName("tokenizer")]
    public string? Tokenizer { get; set; }

    [JsonPropertyName("instruct_type")]
    public string? InstructType { get; set; }
}
