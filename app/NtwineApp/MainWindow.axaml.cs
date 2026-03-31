using System;
using System.Linq;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using NtwineApp.Services;
using NtwineApp.ViewModels;

namespace NtwineApp;

public partial class MainWindow : Window
{
    private readonly GoBackendProcess _backend = new("8090");
    private MainViewModel? _vm;

    public MainWindow()
    {
        InitializeComponent();

        _vm = new MainViewModel(_backend, this);
        DataContext = _vm;

        Opened += async (_, _) =>
        {
            _vm.StatusText = "starting backend...";
            var ok = await _backend.Start();
            _vm.BackendConnected = ok;

            if (ok)
            {
                _vm.StatusText = "syncing config...";
                await SyncConfigToBackend();
                await FetchToolCount();
                _vm.StatusText = "ready";
            }
            else
            {
                _vm.StatusText = "backend offline - run 'go run ./cmd/server -port 8090' manually";
            }
        };

        Closing += (_, _) =>
        {
            _backend.Dispose();
        };
    }

    private void OnRemoveModelClick(object? sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is string modelId && _vm != null)
        {
            var model = _vm.SelectedModels.FirstOrDefault(m => m.ModelId == modelId);
            if (model != null)
            {
                _vm.RemoveModelCommand.Execute(model);
            }
        }
    }

    private void OnModelRowClick(object? sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is string modelId && _vm != null)
        {
            var model = _vm.FilteredModels.FirstOrDefault(m => m.Id == modelId);
            if (model != null)
            {
                _vm.PickModelCommand.Execute(model);
            }
        }
    }

    public async System.Threading.Tasks.Task<string?> PickFolder()
    {
        var folders = await StorageProvider.OpenFolderPickerAsync(new FolderPickerOpenOptions
        {
            Title = "select project folder",
            AllowMultiple = false
        });

        if (folders.Count > 0)
        {
            var path = folders[0].TryGetLocalPath();
            return path;
        }
        return null;
    }

    private async System.Threading.Tasks.Task SyncConfigToBackend()
    {
        if (_vm == null) return;

        try
        {
            using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(5) };
            var config = new
            {
                api_key = _vm.OpenRouterKey,
                tavily_api_key = _vm.TavilyKey,
                models = _vm.SelectedModels.Select(m => m.ModelId).ToArray(),
                provider_keys = new System.Collections.Generic.Dictionary<string, string>()
            };

            if (!string.IsNullOrEmpty(_vm.AnthropicKey))
                config.provider_keys["anthropic"] = _vm.AnthropicKey;
            if (!string.IsNullOrEmpty(_vm.OpenAIKey))
                config.provider_keys["openai"] = _vm.OpenAIKey;
            if (!string.IsNullOrEmpty(_vm.GoogleKey))
                config.provider_keys["google"] = _vm.GoogleKey;
            if (!string.IsNullOrEmpty(_vm.DeepSeekKey))
                config.provider_keys["deepseek"] = _vm.DeepSeekKey;

            var json = JsonSerializer.Serialize(config);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            await http.PostAsync($"{_backend.BaseUrl}/api/config", content);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[sync] config sync failed: {ex.Message}");
        }
    }

    private async System.Threading.Tasks.Task FetchToolCount()
    {
        if (_vm == null) return;

        try
        {
            using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(3) };
            var resp = await http.GetStringAsync($"{_backend.BaseUrl}/api/tools/count");
            using var doc = JsonDocument.Parse(resp);
            if (doc.RootElement.TryGetProperty("count", out var count))
            {
                _vm.ToolCount = count.GetInt32();
            }
        }
        catch { }
    }
}
