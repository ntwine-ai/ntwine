using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using NtwineApp.ViewModels;

namespace NtwineApp.Services;

public static class ThreadStorageService
{
    private static readonly string ThreadsDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".ntwine", "threads");

    private static readonly JsonSerializerOptions JsonOptions = new() { WriteIndented = true };

    public static void Save(string id, string title, IEnumerable<ChatMessage> messages)
    {
        Directory.CreateDirectory(ThreadsDir);

        var data = new
        {
            id,
            title,
            created_at = DateTime.UtcNow.ToString("o"),
            messages = messages.Select(m => new
            {
                agent_name = m.AgentName,
                agent_color = m.AgentColor,
                content = m.Content,
                timestamp = m.Timestamp
            }).ToList()
        };

        var path = Path.Combine(ThreadsDir, $"{id}.json");
        File.WriteAllText(path, JsonSerializer.Serialize(data, JsonOptions));
    }

    public static (string Title, List<ChatMessage> Messages)? Load(string id)
    {
        var path = Path.Combine(ThreadsDir, $"{id}.json");
        if (!File.Exists(path)) return null;

        try
        {
            var json = File.ReadAllText(path);
            using var doc = JsonDocument.Parse(json);
            var root = doc.RootElement;

            var title = root.GetProperty("title").GetString() ?? "Untitled";
            var msgs = new List<ChatMessage>();

            foreach (var m in root.GetProperty("messages").EnumerateArray())
            {
                var name = m.GetProperty("agent_name").GetString() ?? "unknown";
                var color = m.GetProperty("agent_color").GetString() ?? "#7a6f96";
                var content = m.GetProperty("content").GetString() ?? "";
                msgs.Add(new ChatMessage(name, color, content));
            }

            return (title, msgs);
        }
        catch
        {
            return null;
        }
    }

    public static List<ThreadItem> ListAll(int limit = 20)
    {
        if (!Directory.Exists(ThreadsDir)) return new List<ThreadItem>();

        var threads = new List<(string Id, string Title, DateTime CreatedAt)>();

        foreach (var file in Directory.GetFiles(ThreadsDir, "*.json"))
        {
            try
            {
                var json = File.ReadAllText(file);
                using var doc = JsonDocument.Parse(json);
                var root = doc.RootElement;

                var id = root.GetProperty("id").GetString() ?? Path.GetFileNameWithoutExtension(file);
                var title = root.GetProperty("title").GetString() ?? "Untitled";
                var createdAt = root.TryGetProperty("created_at", out var ca)
                    ? DateTime.Parse(ca.GetString() ?? DateTime.MinValue.ToString("o"))
                    : File.GetCreationTimeUtc(file);

                threads.Add((id, title, createdAt));
            }
            catch
            {
            }
        }

        return threads
            .OrderByDescending(t => t.CreatedAt)
            .Take(limit)
            .Select(t => new ThreadItem(t.Id, t.Title, t.CreatedAt.ToLocalTime().ToString("MMM d")))
            .ToList();
    }

    public static void Delete(string id)
    {
        var path = Path.Combine(ThreadsDir, $"{id}.json");
        if (File.Exists(path)) File.Delete(path);
    }

    public static void ClearAll()
    {
        if (!Directory.Exists(ThreadsDir)) return;

        foreach (var file in Directory.GetFiles(ThreadsDir, "*.json"))
        {
            try { File.Delete(file); }
            catch { }
        }
    }
}
