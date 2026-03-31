using System;
using System.Collections.Generic;
using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using NtwineApp.ViewModels;

namespace NtwineApp.Services;

public class BackendService
{
    private ClientWebSocket? _ws;
    private CancellationTokenSource? _cts;
    private string _backendUrl = "ws://localhost:8090/api/discuss";

    public void SetUrl(string url)
    {
        _backendUrl = url;
    }

    public async Task StartDiscussion(
        string prompt,
        string codebasePath,
        List<string> models,
        int rounds,
        Action<ChatMessage> onMessage,
        Action<string> onNotesUpdate,
        Action onComplete)
    {
        _cts = new CancellationTokenSource();
        _ws = new ClientWebSocket();

        try
        {
            await _ws.ConnectAsync(new Uri(_backendUrl), _cts.Token);

            var request = new
            {
                prompt,
                codebase_path = string.IsNullOrEmpty(codebasePath) ? "." : codebasePath,
                rounds,
                models
            };

            var json = JsonSerializer.Serialize(request);
            var bytes = Encoding.UTF8.GetBytes(json);
            await _ws.SendAsync(bytes, WebSocketMessageType.Text, true, _cts.Token);

            var buffer = new byte[65536];
            while (_ws.State == WebSocketState.Open && !_cts.Token.IsCancellationRequested)
            {
                var result = await _ws.ReceiveAsync(buffer, _cts.Token);
                if (result.MessageType == WebSocketMessageType.Close)
                    break;

                var text = Encoding.UTF8.GetString(buffer, 0, result.Count);
                ProcessEvent(text, onMessage, onNotesUpdate);
            }
        }
        catch (OperationCanceledException) { }
        catch (WebSocketException) { }
        finally
        {
            onComplete();
            if (_ws?.State == WebSocketState.Open)
            {
                try
                {
                    await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None);
                }
                catch { }
            }
            _ws?.Dispose();
            _ws = null;
        }
    }

    public void Stop()
    {
        _cts?.Cancel();
    }

    public async Task SendInject(string content)
    {
        if (_ws?.State != WebSocketState.Open) return;

        var msg = JsonSerializer.Serialize(new { action = "inject", content });
        var bytes = Encoding.UTF8.GetBytes(msg);
        await _ws.SendAsync(bytes, WebSocketMessageType.Text, true, CancellationToken.None);
    }

    public async Task SendMute(string modelId)
    {
        if (_ws?.State != WebSocketState.Open) return;

        var msg = JsonSerializer.Serialize(new { action = "mute", model_id = modelId });
        var bytes = Encoding.UTF8.GetBytes(msg);
        await _ws.SendAsync(bytes, WebSocketMessageType.Text, true, CancellationToken.None);
    }

    public async Task SendUnmute(string modelId)
    {
        if (_ws?.State != WebSocketState.Open) return;

        var msg = JsonSerializer.Serialize(new { action = "unmute", model_id = modelId });
        var bytes = Encoding.UTF8.GetBytes(msg);
        await _ws.SendAsync(bytes, WebSocketMessageType.Text, true, CancellationToken.None);
    }

    private void ProcessEvent(string json, Action<ChatMessage> onMessage, Action<string> onNotesUpdate)
    {
        try
        {
            using var doc = JsonDocument.Parse(json);
            var root = doc.RootElement;

            var type = root.GetProperty("type").GetString() ?? "";
            var modelId = root.TryGetProperty("model_id", out var mid) ? mid.GetString() ?? "" : "";
            var displayName = root.TryGetProperty("display_name", out var dn) ? dn.GetString() ?? "" : "";

            var color = GetColorForModel(displayName);

            switch (type)
            {
                case "message":
                    var content = root.GetProperty("content").GetString() ?? "";
                    onMessage(new ChatMessage(displayName, color, content));
                    break;

                case "tool_call":
                    if (root.GetProperty("content").ValueKind == JsonValueKind.Object)
                    {
                        var tc = root.GetProperty("content");
                        var toolName = tc.GetProperty("name").GetString() ?? "";
                        onMessage(new ChatMessage(displayName, "#7a6f96",
                            $"[tool] {toolName}"));
                    }
                    break;

                case "tool_result":
                    if (root.GetProperty("content").ValueKind == JsonValueKind.Object)
                    {
                        var tr = root.GetProperty("content");
                        var toolName = tr.GetProperty("name").GetString() ?? "";
                        var resultText = tr.GetProperty("result").GetString() ?? "";
                        if (resultText.Length > 200)
                            resultText = resultText[..200] + "...";
                        onMessage(new ChatMessage(displayName, "#7a6f96",
                            $"[result] {toolName}: {resultText}"));
                    }
                    break;

                case "notes_update":
                    var notes = root.GetProperty("content").GetString() ?? "";
                    onNotesUpdate(notes);
                    onMessage(new ChatMessage("spec", "#8b7cf8",
                        "[shared spec updated]"));
                    break;

                case "status":
                    var status = root.GetProperty("content").GetString() ?? "";
                    if (status.Contains("round"))
                    {
                        onMessage(new ChatMessage("system", "#7a6f96", status));
                    }
                    break;

                case "error":
                    var err = root.GetProperty("content").GetString() ?? "";
                    onMessage(new ChatMessage("error", "#ef4444", err));
                    break;

                case "execution_prompt":
                    var ep = root.GetProperty("content").GetString() ?? "";
                    onMessage(new ChatMessage("plan", "#10b981",
                        ep.Length > 500 ? ep[..500] + "..." : ep));
                    break;
            }
        }
        catch { }
    }

    private static string GetColorForModel(string name)
    {
        var lower = name.ToLowerInvariant();
        if (lower.Contains("claude")) return "#8b5cf6";
        if (lower.Contains("gpt")) return "#10b981";
        if (lower.Contains("gemini")) return "#3b82f6";
        if (lower.Contains("deepseek")) return "#06b6d4";
        if (lower.Contains("qwen")) return "#f59e0b";
        if (lower.Contains("minimax")) return "#ec4899";
        if (lower.Contains("grok")) return "#ef4444";
        if (lower.Contains("llama")) return "#84cc16";
        if (lower.Contains("mistral")) return "#a855f7";
        if (lower.Contains("kimi")) return "#f97316";
        return "#b0a4c8";
    }
}
