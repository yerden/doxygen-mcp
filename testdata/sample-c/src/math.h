/**
 * @file math.h
 * @brief Simple math utilities for testing.
 */
#ifndef MATH_H
#define MATH_H

/** Maximum value constant */
#define MAX_VALUE 1000

/** Point in 2D space */
typedef struct {
    int x; /**< X coordinate */
    int y; /**< Y coordinate */
} Point;

/** Color enumeration */
typedef enum {
    COLOR_RED,
    COLOR_GREEN,
    COLOR_BLUE
} Color;

/**
 * @brief Add two integers.
 * @param a First operand.
 * @param b Second operand.
 * @return Sum of a and b.
 */
int add(int a, int b);

/**
 * @brief Multiply two integers.
 * @param a First factor.
 * @param b Second factor.
 * @return Product.
 */
int multiply(int a, int b);

/**
 * @brief Compute Euclidean distance squared between two points.
 * @param p1 First point.
 * @param p2 Second point.
 * @return Distance squared.
 */
int distance_sq(Point p1, Point p2);

#endif /* MATH_H */
