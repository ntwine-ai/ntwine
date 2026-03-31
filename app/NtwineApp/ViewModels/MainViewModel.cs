using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NtwineApp.Services;

namespace NtwineApp.ViewModels;

public partial class MainViewModel : ObservableObject
{
    private readonly OpenRouterService _openRouter = new();
    private readonly BackendService _backend = new();

    [ObservableProperty]
    private string _promptText = "";

    [ObservableProperty]
    private string _projectPath = "";

    [ObservableProperty]
    private string _currentDiscussionTitle = "New Discussion";

    [ObservableProperty]
    private string _creditsRemaining = "$7.00";

    [ObservableProperty]
    private int _agentCount = 3;

    [ObservableProperty]
    private int _toolCount = 15;

    [ObservableProperty]
    private bool _isRunning = false;

    [ObservableProperty]
    private string _statusText = "ready";

    [ObservableProperty]
    private bool _isModelPickerOpen = false;

    [ObservableProperty]
    private string _modelSearchQuery = "";

    [ObservableProperty]
    private string _selectedCategory = "All";

    [ObservableProperty]
    private string _selectedProvider = "All";

    [ObservableProperty]
    private string _sharedNotes = "";

    [ObservableProperty]
    private bool _isSettingsOpen = false;

    [ObservableProperty]
    private string _openRouterKey = "";

    [ObservableProperty]
    private string _tavilyKey = "";

    [ObservableProperty]
    private string _anthropicKey = "";

    [ObservableProperty]
    private string _openAIKey = "";

    [ObservableProperty]
    private string _googleKey = "";

    [ObservableProperty]
    private string _deepSeekKey = "";

    [ObservableProperty]
    private string _backendUrl = "localhost:8080";

    public ObservableCollection<ChatMessage> Messages { get; } = new();
    public ObservableCollection<ModelSlot> SelectedModels { get; } = new();
    public ObservableCollection<ThreadItem> Threads { get; } = new();
    public ObservableCollection<ModelListItem> AvailableModels { get; } = new();
    public ObservableCollection<string> Categories { get; } = new();
    public ObservableCollection<string> Providers { get; } = new();

    public MainViewModel()
    {
        SelectedModels.Add(new ModelSlot("deepseek/deepseek-chat", "DeepSeek V3", "#3b82f6", "$", "free"));
        SelectedModels.Add(new ModelSlot("qwen/qwen3-coder", "Qwen3 Coder", "#f59e0b", "$", "free"));
        SelectedModels.Add(new ModelSlot("google/gemini-2.5-flash", "Gemini Flash", "#10b981", "$", "$0.30/1M"));

        Threads.Add(new ThreadItem("auth system design"));
        Threads.Add(new ThreadItem("fix payment flow"));
        Threads.Add(new ThreadItem("database schema review"));

        _ = LoadModels();
    }

    private async Task LoadModels()
    {
        try
        {
            StatusText = "loading models...";
            var models = await _openRouter.FetchModels();

            AvailableModels.Clear();
            var colorIndex = 0;
            foreach (var m in models)
            {
                AvailableModels.Add(new ModelListItem(
                    m.Id ?? "",
                    m.Name ?? m.ShortName,
                    OpenRouterService.FormatProviderName(m.Provider),
                    m.CostLabel,
                    m.CostDetail,
                    m.IsFree,
                    m.ContextLength,
                    OpenRouterService.GetAgentColor(colorIndex++),
                    m.SupportsVision
                ));
            }

            Categories.Clear();
            foreach (var c in _openRouter.GetCategories())
                Categories.Add(c);

            Providers.Clear();
            Providers.Add("All");
            foreach (var p in _openRouter.GetProviders())
                Providers.Add(OpenRouterService.FormatProviderName(p));

            StatusText = $"{models.Count} models loaded";
        }
        catch (Exception ex)
        {
            StatusText = $"failed to load models: {ex.Message}";
        }
    }

    [RelayCommand]
    private void NewDiscussion()
    {
        Messages.Clear();
        PromptText = "";
        CurrentDiscussionTitle = "New Discussion";
        IsRunning = false;
        StatusText = "ready";
    }

    [RelayCommand]
    private async Task StartDiscussion()
    {
        if (string.IsNullOrWhiteSpace(PromptText)) return;
        if (SelectedModels.Count == 0) return;

        IsRunning = true;
        CurrentDiscussionTitle = PromptText.Length > 40
            ? PromptText[..40] + "..."
            : PromptText;

        Messages.Add(new ChatMessage("You", "#b0a4c8", PromptText, DateTime.Now));

        var modelIds = SelectedModels.Select(m => m.ModelId).ToList();
        StatusText = $"discussing with {modelIds.Count} models...";

        try
        {
            await _backend.StartDiscussion(
                PromptText,
                ProjectPath,
                modelIds,
                5,
                msg =>
                {
                    Avalonia.Threading.Dispatcher.UIThread.Post(() =>
                    {
                        Messages.Add(msg);
                        StatusText = $"round {Messages.Count / modelIds.Count + 1}";
                    });
                },
                notes =>
                {
                    Avalonia.Threading.Dispatcher.UIThread.Post(() =>
                    {
                        SharedNotes = notes;
                    });
                },
                () =>
                {
                    Avalonia.Threading.Dispatcher.UIThread.Post(() =>
                    {
                        IsRunning = false;
                        StatusText = "done";
                    });
                }
            );
        }
        catch (Exception ex)
        {
            Messages.Add(new ChatMessage("system", "#ef4444", $"error: {ex.Message}", DateTime.Now));
            IsRunning = false;
            StatusText = "error";
        }

        PromptText = "";
    }

    [RelayCommand]
    private void StopDiscussion()
    {
        _backend.Stop();
        IsRunning = false;
        StatusText = "stopped";
    }

    [RelayCommand]
    private void ToggleModelPicker()
    {
        IsModelPickerOpen = !IsModelPickerOpen;
    }

    [RelayCommand]
    private void AddModel(ModelListItem model)
    {
        if (SelectedModels.Any(m => m.ModelId == model.Id)) return;
        if (SelectedModels.Count >= 5)
        {
            StatusText = "max 5 models";
            return;
        }

        SelectedModels.Add(new ModelSlot(
            model.Id,
            model.Name,
            OpenRouterService.GetAgentColor(SelectedModels.Count),
            model.CostLabel,
            model.CostDetail
        ));
        AgentCount = SelectedModels.Count;
        IsModelPickerOpen = false;
    }

    [RelayCommand]
    private void RemoveModel(ModelSlot model)
    {
        SelectedModels.Remove(model);
        AgentCount = SelectedModels.Count;
    }

    [RelayCommand]
    private async Task SelectProject()
    {
        // will be wired to folder picker
        StatusText = "select a project folder...";
    }

    partial void OnModelSearchQueryChanged(string value)
    {
        FilterModels();
    }

    partial void OnSelectedCategoryChanged(string value)
    {
        FilterModels();
    }

    partial void OnSelectedProviderChanged(string value)
    {
        FilterModels();
    }

    private void FilterModels()
    {
        var all = _openRouter.GetCached();

        if (!string.IsNullOrWhiteSpace(ModelSearchQuery))
            all = _openRouter.Search(ModelSearchQuery);

        if (SelectedCategory != "All")
            all = _openRouter.FilterByCategory(SelectedCategory);

        AvailableModels.Clear();
        var colorIndex = 0;
        foreach (var m in all)
        {
            if (SelectedProvider != "All" &&
                OpenRouterService.FormatProviderName(m.Provider) != SelectedProvider)
                continue;

            AvailableModels.Add(new ModelListItem(
                m.Id ?? "",
                m.Name ?? m.ShortName,
                OpenRouterService.FormatProviderName(m.Provider),
                m.CostLabel,
                m.CostDetail,
                m.IsFree,
                m.ContextLength,
                OpenRouterService.GetAgentColor(colorIndex++),
                m.SupportsVision
            ));
        }
    }
}

public class ChatMessage
{
    public string AgentName { get; }
    public string AgentColor { get; }
    public string Content { get; }
    public string Timestamp { get; }

    public ChatMessage(string name, string color, string content, DateTime time)
    {
        AgentName = name;
        AgentColor = color;
        Content = content;
        Timestamp = time.ToString("HH:mm");
    }
}

public class ModelSlot
{
    public string ModelId { get; }
    public string DisplayName { get; }
    public string Color { get; }
    public string CostLabel { get; }
    public string CostDetail { get; }

    public ModelSlot(string id, string name, string color, string cost, string costDetail)
    {
        ModelId = id;
        DisplayName = name;
        Color = color;
        CostLabel = cost;
        CostDetail = costDetail;
    }
}

public class ModelListItem
{
    public string Id { get; }
    public string Name { get; }
    public string Provider { get; }
    public string CostLabel { get; }
    public string CostDetail { get; }
    public bool IsFree { get; }
    public int ContextLength { get; }
    public string Color { get; }
    public bool SupportsVision { get; }
    public string ContextDisplay => ContextLength >= 1_000_000
        ? $"{ContextLength / 1_000_000}M"
        : $"{ContextLength / 1000}K";

    public ModelListItem(string id, string name, string provider, string costLabel,
        string costDetail, bool isFree, int contextLength, string color, bool supportsVision)
    {
        Id = id;
        Name = name;
        Provider = provider;
        CostLabel = costLabel;
        CostDetail = costDetail;
        IsFree = isFree;
        ContextLength = contextLength;
        Color = color;
        SupportsVision = supportsVision;
    }
}

public class ThreadItem
{
    public string Title { get; }
    public ThreadItem(string title) => Title = title;
}
