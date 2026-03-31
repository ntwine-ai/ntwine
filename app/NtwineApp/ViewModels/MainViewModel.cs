using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Avalonia.Media;
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

    [ObservableProperty] private int _activeAgentIndex = 0;
    [ObservableProperty] private string _activeAgentName = "no agents";
    [ObservableProperty] private string _activeAgentColor = "#7a6f96";
    [ObservableProperty] private IBrush _activeAgentBrush = new SolidColorBrush(Color.Parse("#7a6f96"));
    [ObservableProperty] private string _permissionMode = "plan";

    [ObservableProperty] private bool _isSettingsOpen = false;
    [ObservableProperty] private bool _isModelPickerOpen = false;
    [ObservableProperty] private bool _hasNotes = false;
    [ObservableProperty] private string _modelSearchQuery = "";
    [ObservableProperty] private string _selectedProviderFilter = "All";
    [ObservableProperty] private bool _showFreeOnly = false;
    [ObservableProperty] private bool _isLoadingModels = false;

    [ObservableProperty] private bool _byokEnabled = false;
    [ObservableProperty] private string _newKeyProvider = "OpenRouter";

    // kept for backward compat with settings loading
    [ObservableProperty] private string _openRouterKey = "";
    [ObservableProperty] private string _tavilyKey = "";
    [ObservableProperty] private string _anthropicKey = "";
    [ObservableProperty] private string _openAIKey = "";
    [ObservableProperty] private string _googleKey = "";
    [ObservableProperty] private string _deepSeekKey = "";

    public ObservableCollection<ApiKeyEntry> ApiKeys { get; } = new();
    public ObservableCollection<string> AvailableProviders { get; } = new()
    {
        "OpenRouter", "Anthropic", "OpenAI", "Google", "DeepSeek",
        "Mistral", "Cohere", "xAI", "Together", "Fireworks", "Groq"
    };

    public ObservableCollection<ChatMessage> Messages { get; }

    public bool HasMessages => Messages.Count > 0;
    public ObservableCollection<ModelSlot> SelectedModels { get; } = new();
    public ObservableCollection<ThreadItem> Threads { get; } = new();
    public ObservableCollection<PickerModel> FilteredModels { get; } = new();
    public ObservableCollection<ProviderItem> ProviderFilters { get; } = new();

    private static readonly string SettingsDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".ntwine");
    private static readonly string SettingsPath = Path.Combine(SettingsDir, "app-settings.json");

    public MainViewModel(GoBackendProcess goBackend, MainWindow? window = null)
    {
        _goBackend = goBackend;
        _window = window;

        Messages = new ObservableCollection<ChatMessage>();
        Messages.CollectionChanged += (_, _) => OnPropertyChanged(nameof(HasMessages));

        LoadSettings();

        if (SelectedModels.Count == 0)
        {
            SelectedModels.Add(new ModelSlot("deepseek/deepseek-chat", "DeepSeek V3", "#3b82f6", "free"));
            SelectedModels.Add(new ModelSlot("qwen/qwen3-coder", "Qwen3 Coder", "#f59e0b", "free"));
            SelectedModels.Add(new ModelSlot("google/gemini-2.5-flash-preview", "Gemini Flash", "#10b981", "$0.30/1M"));
        }
        AgentCount = SelectedModels.Count;
        UpdateActiveAgent();
        OnPermissionModeChanged(PermissionMode);
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
                5,
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
        UpdateActiveAgent();
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
        UpdateActiveAgent();
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

    private static readonly string[] ModeOrder = { "plan", "accept-edits", "allow-all", "custom" };
    private static readonly Dictionary<string, string> ModeDisplayNames = new()
    {
        ["plan"] = "Plan", ["accept-edits"] = "Accept Edits", ["allow-all"] = "Allow All", ["custom"] = "Custom"
    };
    private static readonly Dictionary<string, string> ModeColors = new()
    {
        ["plan"] = "#8b7cf8", ["accept-edits"] = "#10b981", ["allow-all"] = "#f59e0b", ["custom"] = "#a855f7"
    };

    [ObservableProperty] private string _activeModeDisplayName = "Plan";
    [ObservableProperty] private string _activeModeColor = "#8b7cf8";
    [ObservableProperty] private IBrush _activeModeBrush = new SolidColorBrush(Color.Parse("#8b7cf8"));

    [RelayCommand]
    private void CycleMode()
    {
        var idx = Array.IndexOf(ModeOrder, PermissionMode);
        PermissionMode = ModeOrder[(idx + 1) % ModeOrder.Length];
    }

    partial void OnPermissionModeChanged(string value)
    {
        ActiveModeDisplayName = ModeDisplayNames.GetValueOrDefault(value, value);
        ActiveModeColor = ModeColors.GetValueOrDefault(value, "#7a6f96");
        try { ActiveModeBrush = new SolidColorBrush(Color.Parse(ActiveModeColor)); }
        catch { ActiveModeBrush = new SolidColorBrush(Color.Parse("#7a6f96")); }
    }

    [RelayCommand]
    private void CycleAgent()
    {
        if (SelectedModels.Count == 0) return;
        ActiveAgentIndex = (ActiveAgentIndex + 1) % SelectedModels.Count;
        UpdateActiveAgent();
    }

    private void UpdateActiveAgent()
    {
        if (SelectedModels.Count == 0)
        {
            ActiveAgentName = "no agents";
            ActiveAgentColor = "#7a6f96";
            ActiveAgentBrush = new SolidColorBrush(Color.Parse("#7a6f96"));
            return;
        }
        var idx = Math.Clamp(ActiveAgentIndex, 0, SelectedModels.Count - 1);
        ActiveAgentIndex = idx;
        ActiveAgentName = SelectedModels[idx].DisplayName;
        ActiveAgentColor = SelectedModels[idx].Color;
        try { ActiveAgentBrush = new SolidColorBrush(Color.Parse(SelectedModels[idx].Color)); }
        catch { ActiveAgentBrush = new SolidColorBrush(Color.Parse("#7a6f96")); }
    }

    [RelayCommand]
    private void AddApiKey()
    {
        if (string.IsNullOrEmpty(NewKeyProvider)) return;
        if (ApiKeys.Any(k => k.Provider == NewKeyProvider)) return;

        ApiKeys.Add(new ApiKeyEntry(NewKeyProvider, "", GetPlaceholder(NewKeyProvider)));
    }

    private static string GetPlaceholder(string provider) => provider switch
    {
        "OpenRouter" => "sk-or-...",
        "Anthropic" => "sk-ant-...",
        "OpenAI" => "sk-...",
        "Google" => "AIza...",
        "DeepSeek" => "sk-...",
        "Mistral" => "...",
        "xAI" => "xai-...",
        _ => "..."
    };

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
    private async Task ShowAddModel()
    {
        IsModelPickerOpen = !IsModelPickerOpen;
        if (IsModelPickerOpen && FilteredModels.Count == 0)
        {
            await LoadModels();
        }
    }

    [RelayCommand]
    private void PickModel(PickerModel? model)
    {
        if (model == null) return;
        if (SelectedModels.Any(m => m.ModelId == model.Id)) return;
        if (SelectedModels.Count >= 5)
        {
            StatusText = "max 5 models";
            return;
        }

        var cleanName = CleanModelName(model.Name);
        SelectedModels.Add(new ModelSlot(model.Id, cleanName, OpenRouterService.GetAgentColor(SelectedModels.Count), model.CostDetail));
        AgentCount = SelectedModels.Count;
        UpdateActiveAgent();
        IsModelPickerOpen = false;
        ModelSearchQuery = "";
        SaveSettingsInternal();
    }

    private async Task LoadModels()
    {
        IsLoadingModels = true;
        StatusText = "loading models...";

        try
        {
            var models = await _openRouter.FetchModels();

            ProviderFilters.Clear();
            ProviderFilters.Add(new ProviderItem("All", "", "#8b7cf8"));
            ProviderFilters.Add(new ProviderItem("Free", "", "#10b981"));
            foreach (var slug in _openRouter.GetProviders())
            {
                var name = OpenRouterService.FormatProviderName(slug);
                var color = ProviderLogoService.GetProviderColor(slug);
                var item = new ProviderItem(name, slug, color);
                ProviderFilters.Add(item);
                _ = Task.Run(async () =>
                {
                    var logo = await ProviderLogoService.LoadLogo(slug);
                    if (logo != null)
                        Dispatcher.UIThread.Post(() => item.Logo = logo);
                });
            }

            ApplyModelFilters();
            StatusText = $"{models.Count} models loaded";
        }
        catch (Exception ex)
        {
            StatusText = $"failed: {ex.Message}";
        }

        IsLoadingModels = false;
    }

    partial void OnModelSearchQueryChanged(string value) => ApplyModelFilters();
    partial void OnSelectedProviderFilterChanged(string value) => ApplyModelFilters();
    partial void OnShowFreeOnlyChanged(bool value) => ApplyModelFilters();

    private void ApplyModelFilters()
    {
        var all = _openRouter.GetCached();

        if (!string.IsNullOrWhiteSpace(ModelSearchQuery))
            all = _openRouter.Search(ModelSearchQuery);

        if (SelectedProviderFilter == "Free")
            all = all.Where(m => m.IsFree).ToList();
        else if (SelectedProviderFilter != "All")
            all = all.Where(m => OpenRouterService.FormatProviderName(m.Provider) == SelectedProviderFilter).ToList();

        if (ShowFreeOnly)
            all = all.Where(m => m.IsFree).ToList();

        var alreadySelected = SelectedModels.Select(m => m.ModelId).ToHashSet();

        FilteredModels.Clear();
        foreach (var m in all.Take(50))
        {
            FilteredModels.Add(new PickerModel(
                m.Id ?? "",
                m.Name ?? m.ShortName,
                OpenRouterService.FormatProviderName(m.Provider),
                m.CostLabel,
                OpenRouterService.GetCostPerMillion(m),
                m.IsFree,
                m.ContextLength,
                alreadySelected.Contains(m.Id ?? "")
            ));
        }
    }

    private static string CleanModelName(string name)
    {
        var clean = name;
        var colon = clean.IndexOf(':');
        if (colon > 0 && colon < 20) clean = clean[(colon + 1)..].Trim();
        clean = clean.Replace("(free)", "").Replace("(Free)", "").Trim();
        if (clean.Length > 30) clean = clean[..30];
        return clean;
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
                permission_mode = PermissionMode,
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
            if (root.TryGetProperty("permission_mode", out var pm)) PermissionMode = pm.GetString() ?? "plan";
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

public class ApiKeyEntry : ObservableObject
{
    public string Provider { get; }
    public string Placeholder { get; }

    private string _key;
    public string Key
    {
        get => _key;
        set => SetProperty(ref _key, value);
    }

    public ApiKeyEntry(string provider, string key, string placeholder)
    {
        Provider = provider;
        _key = key;
        Placeholder = placeholder;
    }
}

public class PickerModel
{
    public string Id { get; }
    public string Name { get; }
    public string Provider { get; }
    public string CostLabel { get; }
    public string CostDetail { get; }
    public bool IsFree { get; }
    public int ContextLength { get; }
    public bool AlreadySelected { get; }
    public string ContextDisplay => ContextLength >= 1_000_000 ? $"{ContextLength / 1_000_000}M" : $"{ContextLength / 1000}K";
    public string CostColor => IsFree ? "#10b981" : CostLabel == "$" ? "#f59e0b" : CostLabel == "$$" ? "#f97316" : "#ef4444";

    public PickerModel(string id, string name, string provider, string costLabel, string costDetail, bool isFree, int contextLength, bool alreadySelected)
    {
        Id = id;
        Name = name;
        Provider = provider;
        CostLabel = costLabel;
        CostDetail = costDetail;
        IsFree = isFree;
        ContextLength = contextLength;
        AlreadySelected = alreadySelected;
    }
}

public class ProviderItem : ObservableObject
{
    public string Name { get; }
    public string Slug { get; }
    public string Initial => string.IsNullOrEmpty(Name) ? "?" : Name[..1].ToUpper();
    public string InitialColor { get; }

    private Avalonia.Media.Imaging.Bitmap? _logo;
    public Avalonia.Media.Imaging.Bitmap? Logo
    {
        get => _logo;
        set => SetProperty(ref _logo, value);
    }

    public ProviderItem(string name, string slug, string color = "#7a6f96")
    {
        Name = name;
        Slug = slug;
        InitialColor = color;
    }
}
