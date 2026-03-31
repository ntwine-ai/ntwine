using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Avalonia.Threading;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NtwineApp.Services;

namespace NtwineApp.ViewModels;

public partial class MainViewModel : ObservableObject
{
    private readonly OpenRouterService _openRouter = new();
    private readonly BackendService _backend = new();
    private readonly GoBackendProcess _goBackend;
    private readonly MainWindow? _window;

    [ObservableProperty] private string _promptText = "";
    [ObservableProperty] private string _projectPath = "";
    [ObservableProperty] private string _currentDiscussionTitle = "New Discussion";
    [ObservableProperty] private int _agentCount = 0;
    [ObservableProperty] private int _toolCount = 0;
    [ObservableProperty] private bool _isRunning = false;
    [ObservableProperty] private string _statusText = "starting...";
    [ObservableProperty] private bool _backendConnected = false;
    [ObservableProperty] private string _sharedNotes = "";
    [ObservableProperty] private int _rounds = 5;

    [ObservableProperty] private bool _isSettingsOpen = false;
    [ObservableProperty] private bool _hasNotes = false;

    [ObservableProperty] private string _openRouterKey = "";
    [ObservableProperty] private string _tavilyKey = "";
    [ObservableProperty] private string _anthropicKey = "";
    [ObservableProperty] private string _openAIKey = "";
    [ObservableProperty] private string _googleKey = "";
    [ObservableProperty] private string _deepSeekKey = "";

    public ObservableCollection<ChatMessage> Messages { get; } = new();
    public ObservableCollection<ModelSlot> SelectedModels { get; } = new();
    public ObservableCollection<ThreadItem> Threads { get; } = new();

    private static readonly string SettingsDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".ntwine");
    private static readonly string SettingsPath = Path.Combine(SettingsDir, "app-settings.json");

    public MainViewModel(GoBackendProcess goBackend, MainWindow? window = null)
    {
        _goBackend = goBackend;
        _window = window;

        LoadSettings();

        if (SelectedModels.Count == 0)
        {
            SelectedModels.Add(new ModelSlot("deepseek/deepseek-chat", "DeepSeek V3", "#3b82f6", "free"));
            SelectedModels.Add(new ModelSlot("qwen/qwen3-coder", "Qwen3 Coder", "#f59e0b", "free"));
            SelectedModels.Add(new ModelSlot("google/gemini-2.5-flash-preview", "Gemini Flash", "#10b981", "$0.30/1M"));
        }
        AgentCount = SelectedModels.Count;
    }

    public MainViewModel() : this(new GoBackendProcess()) { }

    [RelayCommand]
    private void NewDiscussion()
    {
        Messages.Clear();
        PromptText = "";
        SharedNotes = "";
        CurrentDiscussionTitle = "New Discussion";
        IsRunning = false;
        StatusText = BackendConnected ? "ready" : "backend offline";
    }

    [RelayCommand]
    private async Task StartDiscussion()
    {
        if (string.IsNullOrWhiteSpace(PromptText)) return;

        if (PromptText.StartsWith("/add "))
        {
            var modelId = PromptText[5..].Trim();
            AddModelById(modelId);
            PromptText = "";
            return;
        }

        if (PromptText.StartsWith("/remove "))
        {
            var modelName = PromptText[8..].Trim().ToLowerInvariant();
            var toRemove = SelectedModels.FirstOrDefault(m =>
                m.DisplayName.ToLowerInvariant().Contains(modelName) ||
                m.ModelId.ToLowerInvariant().Contains(modelName));
            if (toRemove != null) RemoveModel(toRemove);
            PromptText = "";
            return;
        }

        if (SelectedModels.Count == 0)
        {
            StatusText = "add at least one model";
            return;
        }

        if (!BackendConnected)
        {
            StatusText = "backend not connected";
            return;
        }

        IsRunning = true;
        var prompt = PromptText;
        PromptText = "";

        CurrentDiscussionTitle = prompt.Length > 50 ? prompt[..50] + "..." : prompt;
        Messages.Add(new ChatMessage("You", "#b0a4c8", prompt));

        var modelIds = SelectedModels.Select(m => m.ModelId).ToList();
        StatusText = $"starting with {modelIds.Count} models...";

        _backend.SetUrl($"ws://localhost:8090/api/discuss");

        try
        {
            await _backend.StartDiscussion(
                prompt,
                string.IsNullOrEmpty(ProjectPath) ? "." : ProjectPath,
                modelIds,
                Rounds,
                msg => Dispatcher.UIThread.Post(() =>
                {
                    Messages.Add(msg);
                    if (msg.AgentName != "system" && msg.AgentName != "spec" && msg.AgentName != "error")
                        StatusText = $"{msg.AgentName} is talking...";
                }),
                notes => Dispatcher.UIThread.Post(() => SharedNotes = notes),
                () => Dispatcher.UIThread.Post(() =>
                {
                    IsRunning = false;
                    StatusText = "done";
                    if (!string.IsNullOrEmpty(CurrentDiscussionTitle))
                        Threads.Insert(0, new ThreadItem(CurrentDiscussionTitle));
                    SaveSettingsInternal();
                })
            );
        }
        catch (Exception ex)
        {
            Messages.Add(new ChatMessage("error", "#ef4444", ex.Message));
            IsRunning = false;
            StatusText = "error";
        }
    }

    [RelayCommand]
    private void StopDiscussion()
    {
        _backend.Stop();
        IsRunning = false;
        StatusText = "stopped";
        Messages.Add(new ChatMessage("system", "#7a6f96", "stopped by user"));
    }

    [RelayCommand]
    private async Task SelectProject()
    {
        if (_window == null) return;

        var path = await _window.PickFolder();
        if (path != null)
        {
            ProjectPath = path;
            StatusText = $"project: {Path.GetFileName(path)}";
            SaveSettingsInternal();
        }
    }

    [RelayCommand]
    private void RemoveModel(ModelSlot? model)
    {
        if (model == null) return;
        SelectedModels.Remove(model);
        AgentCount = SelectedModels.Count;
        SaveSettingsInternal();
    }

    [RelayCommand]
    private void AddModelById(string? modelId)
    {
        if (string.IsNullOrEmpty(modelId)) return;
        if (SelectedModels.Any(m => m.ModelId == modelId)) return;
        if (SelectedModels.Count >= 5)
        {
            StatusText = "max 5 models";
            return;
        }

        var parts = modelId.Split('/');
        var name = parts.Length > 1 ? parts[1] : modelId;
        var color = OpenRouterService.GetAgentColor(SelectedModels.Count);

        SelectedModels.Add(new ModelSlot(modelId, name, color, ""));
        AgentCount = SelectedModels.Count;
        SaveSettingsInternal();
    }

    public string ProjectDisplayName => string.IsNullOrEmpty(ProjectPath) ? "click to select..." : Path.GetFileName(ProjectPath);

    partial void OnProjectPathChanged(string value)
    {
        OnPropertyChanged(nameof(ProjectDisplayName));
    }

    partial void OnSharedNotesChanged(string value)
    {
        HasNotes = !string.IsNullOrWhiteSpace(value);
    }

    [RelayCommand]
    private void ToggleSettings()
    {
        IsSettingsOpen = !IsSettingsOpen;
    }

    [RelayCommand]
    private void SaveSettings()
    {
        SaveSettingsInternal();
        IsSettingsOpen = false;
        StatusText = "settings saved";
    }

    [RelayCommand]
    private void ShowAddModel()
    {
        StatusText = "type model ID (e.g. anthropic/claude-sonnet-4-6) in prompt and prefix with /add";
    }

    [RelayCommand]
    private void IncrementRounds()
    {
        if (Rounds < 20) Rounds++;
    }

    [RelayCommand]
    private void DecrementRounds()
    {
        if (Rounds > 1) Rounds--;
    }

    private void SaveSettingsInternal()
    {
        try
        {
            Directory.CreateDirectory(SettingsDir);
            var data = new
            {
                models = SelectedModels.Select(m => new { m.ModelId, m.DisplayName, m.Color, m.CostDetail }).ToList(),
                project_path = ProjectPath,
                rounds = Rounds,
                openrouter_key = OpenRouterKey,
                tavily_key = TavilyKey,
                anthropic_key = AnthropicKey,
                openai_key = OpenAIKey,
                google_key = GoogleKey,
                deepseek_key = DeepSeekKey
            };
            File.WriteAllText(SettingsPath, JsonSerializer.Serialize(data, new JsonSerializerOptions { WriteIndented = true }));
        }
        catch { }
    }

    private void LoadSettings()
    {
        try
        {
            if (!File.Exists(SettingsPath)) return;
            var json = File.ReadAllText(SettingsPath);
            using var doc = JsonDocument.Parse(json);
            var root = doc.RootElement;

            if (root.TryGetProperty("project_path", out var pp)) ProjectPath = pp.GetString() ?? "";
            if (root.TryGetProperty("rounds", out var r)) Rounds = r.GetInt32();
            if (root.TryGetProperty("openrouter_key", out var ork)) OpenRouterKey = ork.GetString() ?? "";
            if (root.TryGetProperty("tavily_key", out var tk)) TavilyKey = tk.GetString() ?? "";
            if (root.TryGetProperty("anthropic_key", out var ak)) AnthropicKey = ak.GetString() ?? "";
            if (root.TryGetProperty("openai_key", out var oaik)) OpenAIKey = oaik.GetString() ?? "";
            if (root.TryGetProperty("google_key", out var gk)) GoogleKey = gk.GetString() ?? "";
            if (root.TryGetProperty("deepseek_key", out var dsk)) DeepSeekKey = dsk.GetString() ?? "";

            if (root.TryGetProperty("models", out var models))
            {
                SelectedModels.Clear();
                foreach (var m in models.EnumerateArray())
                {
                    SelectedModels.Add(new ModelSlot(
                        m.GetProperty("ModelId").GetString() ?? "",
                        m.GetProperty("DisplayName").GetString() ?? "",
                        m.GetProperty("Color").GetString() ?? "#b0a4c8",
                        m.TryGetProperty("CostDetail", out var cd) ? cd.GetString() ?? "" : ""
                    ));
                }
            }
        }
        catch { }
    }
}

public class ChatMessage
{
    public string AgentName { get; }
    public string AgentColor { get; }
    public string Content { get; }
    public string Timestamp { get; }
    public bool IsToolCall { get; }
    public bool IsError { get; }
    public bool IsSystem { get; }

    public string BackgroundColor { get; }
    public string TextColor { get; }

    public ChatMessage(string name, string color, string content)
    {
        AgentName = name;
        AgentColor = color;
        Content = content;
        Timestamp = DateTime.Now.ToString("HH:mm");
        IsToolCall = content.StartsWith("[tool]") || content.StartsWith("[result]");
        IsError = name == "error";
        IsSystem = name == "system" || name == "spec";

        if (IsError)
        {
            BackgroundColor = "#2a1520";
            TextColor = "#f87171";
        }
        else if (IsToolCall)
        {
            BackgroundColor = "#0f1a1f";
            TextColor = "#7a8f96";
        }
        else if (IsSystem)
        {
            BackgroundColor = "#12101a";
            TextColor = "#7a6f96";
        }
        else if (name == "You")
        {
            BackgroundColor = "#2a1f45";
            TextColor = "#e8e0f4";
        }
        else
        {
            BackgroundColor = "#1e1730";
            TextColor = "#e8e0f4";
        }
    }
}

public class ModelSlot
{
    public string ModelId { get; }
    public string DisplayName { get; }
    public string Color { get; }
    public string CostDetail { get; }

    public ModelSlot(string id, string name, string color, string costDetail)
    {
        ModelId = id;
        DisplayName = name;
        Color = color;
        CostDetail = costDetail;
    }
}

public class ThreadItem
{
    public string Title { get; }
    public ThreadItem(string title) => Title = title;
}
