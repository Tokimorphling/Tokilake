import React, { useState, useEffect, useMemo } from 'react';
import { Box, useTheme, alpha } from '@mui/material';

const TokilakeAnimation = () => {
  const theme = useTheme();
  const isDark = theme.palette.mode === 'dark';
  const color = isDark ? theme.palette.primary.main : theme.palette.primary.dark;

  const [time, setTime] = useState(new Date());

  useEffect(() => {
    const timer = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  const getHandAngles = () => {
    const seconds = time.getSeconds();
    const minutes = time.getMinutes();
    const hours = time.getHours() % 12;
    return {
      second: seconds * 6,
      minute: minutes * 6 + seconds * 0.1,
      hour: hours * 30 + minutes * 0.5,
    };
  };

  const angles = getHandAngles();

  // Generate some random raindrops - Memoize to prevent re-generation on every tick
  const raindrops = useMemo(() => Array.from({ length: 45 }).map((_, i) => ({
    id: i,
    left: `${Math.random() * 120 - 10}%`,
    delay: `${Math.random() * 5}s`,
    duration: `${1.5 + Math.random() * 1.5}s`,
    opacity: 0.1 + Math.random() * 0.3,
  })), []);

  // Generate distorted clocks - Memoize static array
  const clocks = useMemo(() => [
    { id: 1, top: '25%', left: '25%', size: 65, distort: 'skew(5deg, 8deg) scaleX(1.1)' },
    { id: 2, top: '45%', left: '50%', size: 110, distort: 'skew(-10deg, -5deg) scaleY(0.9)' },
    { id: 3, top: '35%', left: '80%', size: 55, distort: 'skew(15deg, -5deg)' },
    { id: 4, top: '68%', left: '32%', size: 85, distort: 'skew(-5deg, 12deg) scaleX(1.2)' },
  ], []);

  return (
    <Box
      sx={{
        width: '100%',
        height: '450px',
        position: 'relative',
        overflow: 'hidden',
        background: 'transparent',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        perspective: '1200px',
        // Consolidate keyframes here to avoid per-element re-parsing
        '@keyframes fall': {
          '0%': { transform: 'translate(0, 0) rotate(20deg)', opacity: 0 },
          '5%': { opacity: 0.3 }, // Base opacity will be multiplied by element-specific opacity
          '90%': { opacity: 0.3 },
          '100%': { transform: 'translate(-150px, 500px) rotate(20deg)', opacity: 0 },
        },
        '@keyframes splash': {
          '0%, 95%': { transform: 'scale(0)', opacity: 0 },
          '97%': { transform: 'scale(1)', opacity: 0.5 },
          '100%': { transform: 'scale(1.5)', opacity: 0 },
        },
        '@keyframes ripple': {
          '0%': { transform: 'scale(0.8)', opacity: 0.5 },
          '100%': { transform: 'scale(2)', opacity: 0 },
        },
        '@keyframes pulse': {
          '0%, 100%': { opacity: 0.5, transform: 'scale(1)' },
          '50%': { opacity: 1, transform: 'scale(1.1)' }
        }
      }}
    >
      {/* Dynamic Celestial Element: Sun or Moon */}
      <Box
        sx={{
          position: 'absolute',
          top: '12%',
          right: '12%',
          width: '60px',
          height: '60px',
          borderRadius: '50%',
          zIndex: 0,
          background: isDark 
            ? 'transparent'
            : `radial-gradient(circle at 30% 30%, ${alpha(theme.palette.warning.light, 0.4)}, transparent)`,
          border: !isDark && `1px solid ${alpha(theme.palette.warning.main, 0.3)}`,
          boxShadow: isDark 
            ? `12px 12px 0 0 ${alpha(color, 0.6)}`
            : `0 0 50px ${alpha(theme.palette.warning.main, 0.4)}`,
          transition: 'all 1s ease-in-out',
          transform: isDark ? 'rotate(-20deg)' : 'none',
          '&::after': !isDark && {
            content: '""',
            position: 'absolute',
            inset: '-20px',
            borderRadius: '50%',
            background: `radial-gradient(circle, ${alpha(theme.palette.warning.main, 0.1)}, transparent 70%)`,
            animation: 'pulse 4s ease-in-out infinite',
          }
        }}
      />

      {/* Rain Effect */}
      <Box sx={{ position: 'absolute', inset: 0, zIndex: 3, pointerEvents: 'none' }}>
        {raindrops.map((rain) => (
          <React.Fragment key={rain.id}>
            <Box
              sx={{
                position: 'absolute',
                top: '-50px',
                left: rain.left,
                width: '2px',
                height: '40px',
                background: `linear-gradient(to bottom, transparent, ${alpha(color, rain.opacity)} 80%, ${color})`,
                borderRadius: '0 0 2px 2px',
                animation: `fall ${rain.duration} linear infinite`,
                animationDelay: rain.delay,
              }}
            />
            <Box
              sx={{
                position: 'absolute',
                bottom: '15%',
                left: rain.left,
                marginLeft: '-100px',
                width: '20px',
                height: '10px',
                borderTop: `1px solid ${alpha(color, 0.4)}`,
                borderRadius: '50%',
                opacity: 0,
                transform: 'scale(0)',
                animation: `splash ${rain.duration} linear infinite`,
                animationDelay: rain.delay,
              }}
            />
          </React.Fragment>
        ))}
      </Box>

      {/* Lake Surface (Tilted) */}
      <Box
        sx={{
          position: 'absolute',
          bottom: '-15%',
          width: '160%',
          height: '65%',
          background: `linear-gradient(to bottom, transparent, ${alpha(color, 0.08)})`,
          transform: 'rotateX(70deg)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1,
          borderTop: `1px solid ${alpha(color, 0.1)}`,
        }}
      >
        {clocks.map((clock) => (
          <Box
            key={clock.id}
            sx={{
              position: 'absolute',
              top: clock.top,
              left: clock.left,
              width: clock.size,
              height: clock.size,
              transform: `translate(-50%, -50%) ${clock.distort}`,
              borderRadius: '45% 55% 50% 50% / 50% 50% 55% 45%',
              border: `1px solid ${alpha(color, 0.4)}`,
              background: `radial-gradient(circle, ${alpha(color, 0.05)}, transparent)`,
              '&::before': {
                content: '""',
                position: 'absolute',
                top: '50%',
                left: '50%',
                width: '3px',
                height: '3px',
                borderRadius: '50%',
                background: color,
                transform: 'translate(-50%, -50%)',
              },
            }}
          >
            <Box
              sx={{
                position: 'absolute',
                top: '50%',
                left: '50%',
                width: '2px',
                height: '25%',
                background: color,
                transformOrigin: 'bottom center',
                transform: `translate(-50%, -100%) rotate(${angles.hour}deg)`,
              }}
            />
            <Box
              sx={{
                position: 'absolute',
                top: '50%',
                left: '50%',
                width: '1.5px',
                height: '35%',
                background: color,
                transformOrigin: 'bottom center',
                transform: `translate(-50%, -100%) rotate(${angles.minute}deg)`,
              }}
            />
            <Box
              sx={{
                position: 'absolute',
                top: '50%',
                left: '50%',
                width: '1px',
                height: '40%',
                background: alpha(color, 0.6),
                transformOrigin: 'bottom center',
                transform: `translate(-50%, -100%) rotate(${angles.second}deg)`,
              }}
            />
            
            <Box
              sx={{
                position: 'absolute',
                inset: '-15px',
                borderRadius: '50%',
                border: `1px solid ${alpha(color, 0.1)}`,
                animation: 'ripple 6s ease-out infinite',
              }}
            />
          </Box>
        ))}
        
        {Array.from({ length: 12 }).map((_, i) => (
          <Box
            key={i}
            sx={{
              position: 'absolute',
              top: `${(i + 1) * 9}%`,
              left: '0',
              right: '0',
              height: '1px',
              background: `linear-gradient(to right, transparent, ${alpha(color, 0.05)}, transparent)`,
            }}
          />
        ))}
      </Box>

      <Box
        sx={{
          position: 'absolute',
          bottom: '50%',
          width: '105%',
          height: '1px',
          background: `linear-gradient(to right, transparent, ${alpha(color, 0.15)}, transparent)`,
          zIndex: 0,
        }}
      />
    </Box>
  );
};

export default TokilakeAnimation;
