using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using System;
using System.IO;
using System.Text.Json;

namespace NtwineApp.ViewModels;

public partial class SettingsViewModel : ObservableObject
{
    private static readonly string ConfigDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        ".ntwine"
    );
    private static readonly string ConfigPath = Path.Combine(ConfigDir, "settings.json");

    [ObservableProperty] private string _openRouterKey = "";
    [ObservableProperty] private string _tavilyKey = "";
    [ObservableProperty] private string _anthropicKey = "";
    [ObservableProperty] private string _openAIKey = "";
    [ObservableProperty] private string _googleKey = "";
    [ObservableProperty] private string _deepSeekKey = "";
    [ObservableProperty] private string _backendUrl = "localhost:8080";

    public SettingsViewModel()
    {
        Load();
    }

    [RelayCommand]
    private void Save()
    {
        try
        {
            Directory.CreateDirectory(ConfigDir);

            var settings = new
            {
                openrouter_key = OpenRouterKey,
                tavily_key = TavilyKey,
                anthropic_key = AnthropicKey,
                openai_key = OpenAIKey,
                google_key = GoogleKey,
                deepseek_key = DeepSeekKey,
                backend_url = BackendUrl
            };

            var json = JsonSerializer.Serialize(settings, new JsonSerializerOptions { WriteIndented = true });
            File.WriteAllText(ConfigPath, json);
        }
        catch { }
    }

    private void Load()
    {
        try
        {
            if (!File.Exists(ConfigPath)) return;

            var json = File.ReadAllText(ConfigPath);
            using var doc = JsonDocument.Parse(json);
            var root = doc.RootElement;

            if (root.TryGetProperty("openrouter_key", out var or)) OpenRouterKey = or.GetString() ?? "";
            if (root.TryGetProperty("tavily_key", out var tv)) TavilyKey = tv.GetString() ?? "";
            if (root.TryGetProperty("anthropic_key", out var an)) AnthropicKey = an.GetString() ?? "";
            if (root.TryGetProperty("openai_key", out var oa)) OpenAIKey = oa.GetString() ?? "";
            if (root.TryGetProperty("google_key", out var go)) GoogleKey = go.GetString() ?? "";
            if (root.TryGetProperty("deepseek_key", out var ds)) DeepSeekKey = ds.GetString() ?? "";
            if (root.TryGetProperty("backend_url", out var bu)) BackendUrl = bu.GetString() ?? "localhost:8080";
        }
        catch { }
    }
}
