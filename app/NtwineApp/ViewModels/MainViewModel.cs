using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Avalonia.Platform.Storage;
using Avalonia.Threading;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NtwineApp.Services;

namespace NtwineApp.ViewModels;

public partial class MainViewModel : ObservableObject
{
    private readonly OpenRouterService _openRouter = new();
    private readonly BackendService _backendSvc = new();
    private readonly GoBackendProcess _goBackend;

    [ObservableProperty] private string _promptText = "";
    [ObservableProperty] private string _projectPath = "";
    [ObservableProperty] private string _currentDiscussionTitle = "New Discussion";
    [ObservableProperty] private string _creditsRemaining = "BYOK";
    [ObservableProperty] private int _agentCount = 0;
    [ObservableProperty] private int _toolCount = 0;
    [ObservableProperty] private bool _isRunning = false;
    [ObservableProperty] private string _statusText = "starting...";
    [ObservableProperty] private bool _backendConnected = false;
    [ObservableProperty] private string _sharedNotes = "";
    [ObservableProperty] private int _rounds = 5;
    [ObservableProperty] private string _mode = "Plan";

    public ObservableCollection<ChatMessage> Messages { get; } = new();
    public ObservableCollection<ModelSlot> SelectedModels { get; } = new();
    public ObservableCollection<ThreadItem> Threads { get; } = new();

    public MainViewModel(GoBackendProcess goBackend)
    {
        _goBackend = goBackend;
        _backendSvc = new BackendService();

        LoadSettings();

        if (SelectedModels.Count == 0)
        {
            SelectedModels.Add(new ModelSlot("deepseek/deepseek-chat", "DeepSeek V3", "#3b82f6", "$", "free"));
            SelectedModels.Add(new ModelSlot("qwen/qwen3-coder", "Qwen3 Coder", "#f59e0b", "$", "free"));
            SelectedModels.Add(new ModelSlot("google/gemini-2.5-flash-preview", "Gemini Flash", "#10b981", "$", "$0.30/1M"));
        }
        AgentCount = SelectedModels.Count;
    }

    public MainViewModel() : this(new GoBackendProcess()) { }

    [RelayCommand]
    private void NewDiscussion()
    {
        Messages.Clear();
        PromptText = "";
        CurrentDiscussionTitle = "New Discussion";
        SharedNotes = "";
        IsRunning = false;
        StatusText = BackendConnected ? "connected" : "backend offline";
    }

    [RelayCommand]
    private async Task StartDiscussion()
    {
        if (string.IsNullOrWhiteSpace(PromptText)) return;
        if (SelectedModels.Count == 0)
        {
            StatusText = "add at least one model first";
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
        Messages.Add(new ChatMessage("You", "#b0a4c8", prompt, DateTime.Now));

        var modelIds = SelectedModels.Select(m => m.ModelId).ToList();
        StatusText = $"starting with {modelIds.Count} models...";

        _backendSvc.SetUrl($"ws://localhost:{8090}/api/discuss");

        try
        {
            await _backendSvc.StartDiscussion(
                prompt,
                string.IsNullOrEmpty(ProjectPath) ? "." : ProjectPath,
                modelIds,
                Rounds,
                msg => Dispatcher.UIThread.Post(() =>
                {
                    Messages.Add(msg);
                    if (msg.AgentName != "system" && msg.AgentName != "spec")
                        StatusText = $"{msg.AgentName} is talking...";
                }),
                notes => Dispatcher.UIThread.Post(() => SharedNotes = notes),
                () => Dispatcher.UIThread.Post(() =>
                {
                    IsRunning = false;
                    StatusText = "done";
                    Threads.Insert(0, new ThreadItem(CurrentDiscussionTitle));
                })
            );
        }
        catch (Exception ex)
        {
            Messages.Add(new ChatMessage("error", "#ef4444", ex.Message, DateTime.Now));
            IsRunning = false;
            StatusText = "error";
        }
    }

    [RelayCommand]
    private void StopDiscussion()
    {
        _backendSvc.Stop();
        IsRunning = false;
        StatusText = "stopped";
        Messages.Add(new ChatMessage("system", "#7a6f96", "discussion stopped by user", DateTime.Now));
    }

    [RelayCommand]
    private void RemoveModel(ModelSlot? model)
    {
        if (model == null) return;
        SelectedModels.Remove(model);
        AgentCount = SelectedModels.Count;
        SaveSettings();
    }

    [RelayCommand]
    private void SetMode(string? newMode)
    {
        if (newMode != null) Mode = newMode;
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

    private static readonly string SettingsDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".ntwine");
    private static readonly string SettingsPath = Path.Combine(SettingsDir, "app-settings.json");

    private void SaveSettings()
    {
        try
        {
            Directory.CreateDirectory(SettingsDir);
            var data = new
            {
                models = SelectedModels.Select(m => new { m.ModelId, m.DisplayName, m.Color, m.CostLabel, m.CostDetail }).ToList(),
                project_path = ProjectPath,
                rounds = Rounds
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

            if (root.TryGetProperty("project_path", out var pp))
                ProjectPath = pp.GetString() ?? "";

            if (root.TryGetProperty("rounds", out var r))
                Rounds = r.GetInt32();

            if (root.TryGetProperty("models", out var models))
            {
                SelectedModels.Clear();
                foreach (var m in models.EnumerateArray())
                {
                    SelectedModels.Add(new ModelSlot(
                        m.GetProperty("ModelId").GetString() ?? "",
                        m.GetProperty("DisplayName").GetString() ?? "",
                        m.GetProperty("Color").GetString() ?? "#b0a4c8",
                        m.GetProperty("CostLabel").GetString() ?? "",
                        m.GetProperty("CostDetail").GetString() ?? ""
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

public class ThreadItem
{
    public string Title { get; }
    public ThreadItem(string title) => Title = title;
}
