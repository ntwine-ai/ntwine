using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace NtwineApp.ViewModels;

public partial class MainViewModel : ObservableObject
{
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

    public ObservableCollection<ChatMessage> Messages { get; } = new();
    public ObservableCollection<ModelSlot> SelectedModels { get; } = new();
    public ObservableCollection<ThreadItem> Threads { get; } = new();

    public MainViewModel()
    {
        SelectedModels.Add(new ModelSlot("Claude Sonnet 4.6", "#8b5cf6", "$$$"));
        SelectedModels.Add(new ModelSlot("DeepSeek V3.2", "#3b82f6", "$"));
        SelectedModels.Add(new ModelSlot("Gemini Flash", "#10b981", "$"));

        Threads.Add(new ThreadItem("auth system design"));
        Threads.Add(new ThreadItem("fix payment flow"));
        Threads.Add(new ThreadItem("database schema review"));
    }

    [RelayCommand]
    private void NewDiscussion()
    {
        Messages.Clear();
        PromptText = "";
        CurrentDiscussionTitle = "New Discussion";
    }

    [RelayCommand]
    private void StartDiscussion()
    {
        if (string.IsNullOrWhiteSpace(PromptText)) return;

        CurrentDiscussionTitle = PromptText.Length > 40
            ? PromptText[..40] + "..."
            : PromptText;

        Messages.Add(new ChatMessage("You", "#b0a4c8", PromptText, "now"));

        Messages.Add(new ChatMessage("Claude", "#8b5cf6", "checking the codebase structure first", "now"));
        Messages.Add(new ChatMessage("DeepSeek", "#3b82f6", "agreed, let me look at the existing patterns", "now"));
        Messages.Add(new ChatMessage("Gemini", "#10b981", "ill check the tests to see what coverage we have", "now"));

        PromptText = "";
    }
}

public class ChatMessage
{
    public string AgentName { get; }
    public string AgentColor { get; }
    public string Content { get; }
    public string Timestamp { get; }
    public string BubbleClass => AgentName == "You" ? "chat-bubble-user" : "chat-bubble";

    public ChatMessage(string name, string color, string content, string timestamp)
    {
        AgentName = name;
        AgentColor = color;
        Content = content;
        Timestamp = timestamp;
    }
}

public class ModelSlot
{
    public string DisplayName { get; }
    public string Color { get; }
    public string CostLabel { get; }

    public ModelSlot(string name, string color, string cost)
    {
        DisplayName = name;
        Color = color;
        CostLabel = cost;
    }
}

public class ThreadItem
{
    public string Title { get; }
    public ThreadItem(string title) => Title = title;
}
