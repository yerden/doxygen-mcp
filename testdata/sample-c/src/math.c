/**
 * @file math.c
 * @brief Math utility implementations.
 */
#include "math.h"

/** Internal helper: clamp value to [0, MAX_VALUE] */
static int clamp(int v) {
    if (v < 0) return 0;
    if (v > MAX_VALUE) return MAX_VALUE;
    return v;
}

int add(int a, int b) {
    return a + b;
}

int multiply(int a, int b) {
    return a * b;
}

int distance_sq(Point p1, Point p2) {
    int dx = p1.x - p2.x;
    int dy = p1.y - p2.y;
    return clamp(dx*dx + dy*dy);
}
