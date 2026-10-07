#include <windows.h>
#include <GL/gl.h>
#include <algorithm>
#include <cstdio>
#include <cstring>
#include <string>

extern void gl_font(int fontid, int size);
extern void gl_draw(const char* value, int x, int y);

static void ink(float r, float g, float b, float a = 1.0f) { glColor4f(r, g, b, a); }
static void box(float x, float y, float w, float h) { glRectf(x, y, x + w, y + h); }
static void line(float x1, float y1, float x2, float y2) {
    glBegin(GL_LINES); glVertex2f(x1, y1); glVertex2f(x2, y2); glEnd();
}
static void label(const char* value, int x, int y, int size) { gl_font(1, size); gl_draw(value, x, y); }
static void icon(const char* value, int x, int y, int size) { gl_font(4, size); gl_draw(value, x, y); }
#if defined(__cpp_char8_t)
static void icon(const char8_t* value, int x, int y, int size) { icon(reinterpret_cast<const char*>(value), x, y, size); }
#endif
static std::string itemAt(const char* values, int index) {
    if (values == nullptr || index < 0) return "";
    const char* start = values;
    for (int i = 0; i < index; ++i) {
        const char* next = std::strchr(start, '\n');
        if (next == nullptr) return "";
        start = next + 1;
    }
    const char* end = std::strchr(start, '\n');
    return end == nullptr ? std::string(start) : std::string(start, end);
}
static void focus(int current, int target, float x, float y, float w, float h) {
    if (current != target) return;
    ink(0.58f, 0.66f, 1.0f);
    glLineWidth(2);
    line(x, y, x + w, y); line(x + w, y, x + w, y + h);
    line(x + w, y + h, x, y + h); line(x, y + h, x, y);
}

extern "C" void ap_overlay_draw(int width, int height, int visible, int menu, int episodes, int volume_open, int playing, int paused, int fullscreen, int selected, int menu_cursor, int volume, int episode, int episode_count, int episode_start, int episode_cursor, int resolution, int loading, int progress_hover, int scrubbing, double position, double duration, double scrub_position, const char* episode_labels, const char* menu_labels) {
    if (!visible && !menu && !episodes && selected < 0) return;
    GLint oldMatrix = GL_MODELVIEW;
    glGetIntegerv(GL_MATRIX_MODE, &oldMatrix);
    glPushAttrib(GL_ENABLE_BIT | GL_COLOR_BUFFER_BIT | GL_CURRENT_BIT | GL_LINE_BIT | GL_VIEWPORT_BIT | GL_SCISSOR_BIT);
    glViewport(0, 0, width, height);
    glDisable(GL_SCISSOR_TEST);
    glMatrixMode(GL_PROJECTION); glPushMatrix(); glLoadIdentity(); glOrtho(0, width, height, 0, -1, 1);
    glMatrixMode(GL_MODELVIEW); glPushMatrix(); glLoadIdentity();
    glDisable(GL_DEPTH_TEST); glDisable(GL_TEXTURE_2D);
    glEnable(GL_BLEND); glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
    const float mid = height - 26.0f;
    ink(0.03f, 0.05f, 0.10f, 0.78f);
    box(0, 0, width, 62); box(0, height - 84, width, 84);
    if (duration > 0) {
        double shown = scrubbing ? scrub_position : position;
        float fraction = static_cast<float>(std::max(0.0, std::min(1.0, shown / duration)));
        ink(0.50f, 0.54f, 0.66f); box(24, height - 69, width - 48, 4);
        ink(0.59f, 0.67f, 1); box(24, height - 69, (width - 48) * fraction, 4);
        if (progress_hover || scrubbing || selected == 6) {
            box(18 + (width - 48) * fraction, height - 74, 12, 14);
        }
        focus(selected, 6, 22, height - 79, width - 44, 24);
    }
    ink(1, 1, 1);
    icon(u8"\uE700", 24, 40, 24);
    focus(selected, 0, 14, 12, 42, 39);
    ink(0.15f, 0.19f, 0.29f, 0.92f); box(width - 195, 14, 114, 38);
    ink(1, 1, 1);
    std::string episodeLabel = episode_count > 0 ? itemAt(episode_labels, episode) : "選集";
    if (episodeLabel.empty()) episodeLabel = "選集";
    label(episodeLabel.c_str(), width - 185, 39, 15);
    icon(u8"\uE70D", width - 102, 39, 14);
    focus(selected, 7, width - 195, 14, 114, 38);
    if (resolution > 0) {
        char quality[24]; std::snprintf(quality, sizeof(quality), "%dp", resolution);
        ink(0.8f, 0.84f, 0.94f); label(quality, width - 68, 39, 14);
    }
    ink(1, 1, 1);
    icon(playing && !paused ? u8"\uE769" : u8"\uE768", 42, static_cast<int>(mid + 9), 24);
    focus(selected, 1, 32, mid - 18, 36, 36);
    icon(u8"\uEB9E", 88, static_cast<int>(mid + 9), 24);
    focus(selected, 2, 82, mid - 18, 36, 36);
    icon(u8"\uEB9D", 138, static_cast<int>(mid + 9), 24);
    focus(selected, 3, 132, mid - 18, 36, 36);
    ink(1, 1, 1);
    icon(u8"\uE71A", 190, static_cast<int>(mid + 9), 24);
    focus(selected, 4, 182, mid - 18, 36, 36);
    ink(1, 1, 1);
    icon(u8"\uE767", 238, static_cast<int>(mid + 9), 24);
    focus(selected, 5, 232, mid - 18, volume_open ? 163 : 40, 36);
    if (volume_open) {
        ink(0.58f, 0.64f, 0.78f); box(286, mid - 2, 100, 4);
        ink(0.63f, 0.7f, 1);
        float level = std::max(0, std::min(100, volume)) / 100.0f;
        box(286, mid - 2, 100 * level, 4); box(282 + 100 * level, mid - 5, 8, 10);
    }
    if (duration > 0) {
        char positionLabel[64];
        int now = static_cast<int>(std::max(0.0, scrubbing ? scrub_position : position)), total = static_cast<int>(duration);
        std::snprintf(positionLabel, sizeof(positionLabel), "%02d:%02d / %02d:%02d", now / 60, now % 60, total / 60, total % 60);
        ink(0.8f, 0.84f, 0.94f); label(positionLabel, volume_open ? 410 : 286, static_cast<int>(mid + 5), 12);
    }
    ink(1, 1, 1);
    icon(fullscreen ? u8"\uE73F" : u8"\uE740", width - 47, static_cast<int>(mid + 9), 24);
    focus(selected, 8, width - 51, mid - 18, 36, 36);
    if (loading) {
        ink(1, 1, 1); label("正在載入…", width / 2 - 42, height / 2, 18);
    }
    if (menu) {
        ink(0.04f, 0.06f, 0.11f, 0.93f); box(0, 62, 180, height - 114);
        ink(0.73f, 0.77f, 0.86f);
        for (int i = 0; i < 6; ++i) {
            if (selected == 0 && menu_cursor == i) { ink(0.22f, 0.27f, 0.44f); box(8, 72 + i * 46, 164, 39); ink(1, 1, 1); }
            std::string item = itemAt(menu_labels, i);
            label(item.c_str(), 26, 110 + i * 46, 15);
        }
    }
    if (episodes) {
        int rows = std::min(episode_count - episode_start, 8);
        ink(0.04f, 0.06f, 0.11f, 0.95f); box(width - 195, 58, 114, 34 * rows + 8);
        for (int i = 0; i < rows; ++i) {
            int entry = episode_start + i;
            if (entry == episode_cursor) { ink(0.22f, 0.27f, 0.44f); box(width - 190, 62 + 34 * i, 104, 32); }
            ink(1, 1, 1);
            std::string item = itemAt(episode_labels, entry);
            label(item.c_str(), width - 180, 84 + i * 34, 14);
        }
    }
    glPopMatrix(); glMatrixMode(GL_PROJECTION); glPopMatrix(); glMatrixMode(oldMatrix); glPopAttrib();
}
